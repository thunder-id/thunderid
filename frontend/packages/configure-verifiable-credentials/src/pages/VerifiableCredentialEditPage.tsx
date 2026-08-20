// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ManagedResourceNotice, PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, IconButton, PageContent, PageTitle, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetVerifiableCredential from '../api/useGetVerifiableCredential';
import useUpdateVerifiableCredential from '../api/useUpdateVerifiableCredential';
import VerifiableCredentialDeleteDialog from '../components/VerifiableCredentialDeleteDialog';
import VerifiableCredentialForm from '../components/VerifiableCredentialForm';
import {useIsManagedResource} from '@thunderid/contexts';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import type {UpdateVerifiableCredentialRequest} from '../models/credential-requests';

export default function VerifiableCredentialEditPage(): JSX.Element {
  const {vcId = ''} = useParams<{vcId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const routes = useVerifiableCredentialRoutes();
  const listUrl = routes.verifiableCredentials.list();

  const {data, isLoading, error, refetch} = useGetVerifiableCredential(vcId);
  const updateVC = useUpdateVerifiableCredential();
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  // Resolves an error through the `verifiable-credentials` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with
  // `verifiable-credentials:`, per getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `verifiable-credentials:${key}`, options),
    [t],
  );
  // Owned by the control plane: a change made here would be replaced by the next apply, and the
  // server refuses it with 403, so the controls are not offered at all.
  const isManaged: boolean = useIsManagedResource('credential_configuration')(vcId);

  const [name, setName] = useState<string>('');
  const [description, setDescription] = useState<string>('');
  const [initializedId, setInitializedId] = useState<string | null>(null);

  const [isEditingName, setIsEditingName] = useState<boolean>(false);
  const [isEditingDescription, setIsEditingDescription] = useState<boolean>(false);
  const [tempName, setTempName] = useState<string>('');
  const [tempDescription, setTempDescription] = useState<string>('');

  // Re-seed the header fields once when a new resource loads (state-during-render, not an effect).
  if (data && data.id !== initializedId) {
    setName(data.name ?? '');
    setDescription(data.description ?? '');
    setInitializedId(data.id);
  }

  const handleDeleted = (): void => {
    void navigate(listUrl);
  };

  const handleSubmit = (formData: UpdateVerifiableCredentialRequest): void => {
    updateVC.mutate({id: vcId, data: formData});
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  const backButton = (
    <Button
      onClick={(): void => {
        void navigate(listUrl);
      }}
      startIcon={<ArrowLeft size={16} />}
    >
      {t('verifiable-credentials:edit.back')}
    </Button>
  );

  if (error) {
    return (
      <PageContent>
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('verifiable-credentials:edit.loadError', 'Failed to load credential template')}
          onRetry={() => void refetch()}
          action={backButton}
        />
      </PageContent>
    );
  }

  if (!data) {
    if (!vcId) {
      return <PageLoadingAnimation />;
    }
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('verifiable-credentials:edit.notFound')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('verifiable-credentials:edit.back')}
        </PageTitle.BackButton>
        <PageTitle.Header component="div">
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                aria-label={t('verifiable-credentials:edit.name.ariaLabel')}
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmed = tempName.trim();
                  if (trimmed && trimmed !== name.trim()) {
                    if (updateVC.isError) updateVC.reset();
                    setName(trimmed);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const trimmed = tempName.trim();
                    if (trimmed && trimmed !== name.trim()) {
                      if (updateVC.isError) updateVC.reset();
                      setName(trimmed);
                    }
                    setIsEditingName(false);
                  } else if (e.key === 'Escape') {
                    setTempName(name);
                    setIsEditingName(false);
                  }
                }}
                size="small"
              />
            ) : (
              <>
                <Typography variant="h3">{name || data.handle}</Typography>
                <IconButton
                  size="small"
                  aria-label={t('verifiable-credentials:edit.name.editButton')}
                  onClick={() => {
                    setTempName(name);
                    setIsEditingName(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}}}
                >
                  <Edit size={16} />
                </IconButton>
              </>
            )}
          </Stack>
        </PageTitle.Header>
        <PageTitle.SubHeader component="div">
          <Stack direction="row" alignItems="flex-start" spacing={1}>
            {isEditingDescription ? (
              <TextField
                aria-label={t('verifiable-credentials:edit.description.ariaLabel')}
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmed = tempDescription.trim();
                  if (trimmed !== description.trim()) {
                    if (updateVC.isError) updateVC.reset();
                    setDescription(trimmed);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmed = tempDescription.trim();
                    if (trimmed !== description.trim()) {
                      if (updateVC.isError) updateVC.reset();
                      setDescription(trimmed);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setTempDescription(description);
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('verifiable-credentials:edit.description.placeholder')}
                sx={{
                  maxWidth: '600px',
                  '& .MuiInputBase-root': {fontSize: '0.875rem'},
                }}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {description || t('verifiable-credentials:edit.description.empty')}
                </Typography>
                <IconButton
                  size="small"
                  aria-label={t('verifiable-credentials:edit.description.editButton')}
                  onClick={() => {
                    setTempDescription(description);
                    setIsEditingDescription(true);
                  }}
                  sx={{opacity: 0.6, '&:hover': {opacity: 1}, mt: -0.5}}
                >
                  <Edit size={14} />
                </IconButton>
              </>
            )}
          </Stack>
        </PageTitle.SubHeader>
      </PageTitle>

      {isManaged && <ManagedResourceNotice />}

      <VerifiableCredentialForm
        initial={data}
        name={name}
        description={description}
        onNameChange={setName}
        onDescriptionChange={setDescription}
        submitting={updateVC.isPending}
        submitLabel={t('common:actions.save')}
        onSubmit={handleSubmit}
        isReadOnly={isManaged}
        onDelete={isManaged ? undefined : (): void => setDeleteOpen(true)}
        error={
          updateVC.error
            ? getErrorMessage(updateVC.error, tForErrors, 'update.error', 'Failed to update credential template')
            : undefined
        }
        onErrorClear={() => {
          if (updateVC.isError) updateVC.reset();
        }}
      />

      <VerifiableCredentialDeleteDialog
        open={deleteOpen}
        vcId={vcId}
        onClose={(): void => setDeleteOpen(false)}
        onSuccess={handleDeleted}
      />
    </PageContent>
  );
}
