// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ManagedResourceNotice, PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Button, IconButton, PageContent, PageTitle, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useCallback, useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetVerifiablePresentation from '../api/useGetVerifiablePresentation';
import useUpdateVerifiablePresentation from '../api/useUpdateVerifiablePresentation';
import VerifiablePresentationDeleteDialog from '../components/VerifiablePresentationDeleteDialog';
import VerifiablePresentationForm from '../components/VerifiablePresentationForm';
import {useIsManagedResource} from '@thunderid/contexts';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';
import type {UpdateVerifiablePresentationRequest} from '../models/presentation-requests';

export default function VerifiablePresentationEditPage(): JSX.Element {
  const {vpId = ''} = useParams<{vpId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const routes = useVerifiableCredentialRoutes();
  const listUrl = routes.verifiablePresentations.list();

  // Resolves an error through the `verifiable-presentations` catalog. `t` defaults to the `common`
  // namespace, so this forwards explicit `ns:` prefixes unchanged and prefixes bare keys, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string =>
      t(key.includes(':') ? key : `verifiable-presentations:${key}`, options),
    [t],
  );

  const {data, isLoading, error, refetch} = useGetVerifiablePresentation(vpId);
  const updateVP = useUpdateVerifiablePresentation();
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  // Owned by the control plane: a change made here would be replaced by the next apply, and the
  // server refuses it with 403, so the controls are not offered at all.
  const isManaged: boolean = useIsManagedResource('presentation_definition')(vpId);

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

  const handleSubmit = (formData: UpdateVerifiablePresentationRequest): void => {
    // Save in place — the form re-snapshots from the refreshed query and the
    // success toast confirms; no navigation away (UnsavedChangesBar pattern).
    updateVP.mutate({id: vpId, data: formData});
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
      {t('verifiable-presentations:edit.back')}
    </Button>
  );

  if (error) {
    return (
      <PageContent>
        <QueryErrorNotice
          error={error}
          t={tForErrors}
          variant="block"
          title={t('verifiable-presentations:edit.loadError', 'Failed to load presentation definition')}
          onRetry={() => void refetch()}
          action={backButton}
        />
      </PageContent>
    );
  }

  if (!data) {
    // A disabled query (no vpId yet) reaches here too — that's not "not found", it just hasn't
    // fetched anything.
    if (!vpId) {
      return <PageLoadingAnimation />;
    }

    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('verifiable-presentations:edit.notFound')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={listUrl} />}>
          {t('verifiable-presentations:edit.back')}
        </PageTitle.BackButton>
        <PageTitle.Header component="div">
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                aria-label={t('verifiable-presentations:edit.name.ariaLabel')}
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmed = tempName.trim();
                  if (trimmed && trimmed !== name.trim()) {
                    if (updateVP.isError) updateVP.reset();
                    setName(trimmed);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const trimmed = tempName.trim();
                    if (trimmed && trimmed !== name.trim()) {
                      if (updateVP.isError) updateVP.reset();
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
                  aria-label={t('verifiable-presentations:edit.name.editButton')}
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
                aria-label={t('verifiable-presentations:edit.description.ariaLabel')}
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmed = tempDescription.trim();
                  if (trimmed !== description.trim()) {
                    if (updateVP.isError) updateVP.reset();
                    setDescription(trimmed);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmed = tempDescription.trim();
                    if (trimmed !== description.trim()) {
                      if (updateVP.isError) updateVP.reset();
                      setDescription(trimmed);
                    }
                    setIsEditingDescription(false);
                  } else if (e.key === 'Escape') {
                    setTempDescription(description);
                    setIsEditingDescription(false);
                  }
                }}
                size="small"
                placeholder={t('verifiable-presentations:edit.description.placeholder')}
                sx={{
                  maxWidth: '600px',
                  '& .MuiInputBase-root': {fontSize: '0.875rem'},
                }}
              />
            ) : (
              <>
                <Typography variant="body2" color="text.secondary">
                  {description || t('verifiable-presentations:edit.description.empty')}
                </Typography>
                <IconButton
                  size="small"
                  aria-label={t('verifiable-presentations:edit.description.editButton')}
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

      <VerifiablePresentationForm
        initial={data}
        name={name}
        description={description}
        onNameChange={setName}
        onDescriptionChange={setDescription}
        submitting={updateVP.isPending}
        submitLabel={t('common:actions.save')}
        onSubmit={handleSubmit}
        error={
          updateVP.error
            ? getErrorMessage(updateVP.error, tForErrors, 'update.error', 'Failed to update presentation definition')
            : undefined
        }
        onErrorClear={() => {
          if (updateVP.isError) updateVP.reset();
        }}
        isReadOnly={isManaged}
        onDelete={isManaged ? undefined : (): void => setDeleteOpen(true)}
      />

      <VerifiablePresentationDeleteDialog
        open={deleteOpen}
        vpId={vpId}
        onClose={(): void => setDeleteOpen(false)}
        onSuccess={handleDeleted}
      />
    </PageContent>
  );
}
