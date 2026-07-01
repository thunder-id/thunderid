/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {PageLoadingAnimation} from '@thunderid/components';
import {Alert, Button, IconButton, PageContent, PageTitle, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {ArrowLeft, Edit} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetVerifiablePresentation from '../api/useGetVerifiablePresentation';
import useUpdateVerifiablePresentation from '../api/useUpdateVerifiablePresentation';
import VerifiablePresentationDeleteDialog from '../components/VerifiablePresentationDeleteDialog';
import VerifiablePresentationForm from '../components/VerifiablePresentationForm';
import type {UpdateVerifiablePresentationRequest} from '../models/requests';

const LIST_URL = '/verifiable-presentations';

export default function VerifiablePresentationEditPage(): JSX.Element {
  const {vpId = ''} = useParams<{vpId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();

  const {data, isLoading, error} = useGetVerifiablePresentation(vpId);
  const updateVP = useUpdateVerifiablePresentation();
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

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
    void navigate(LIST_URL);
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
        void navigate(LIST_URL);
      }}
      startIcon={<ArrowLeft size={16} />}
    >
      {t('verifiable-presentations:edit.back')}
    </Button>
  );

  if (error) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {error.message ?? t('verifiable-presentations:edit.loadError')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  if (!data) {
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
        <PageTitle.BackButton component={<Link to={LIST_URL} />}>
          {t('verifiable-presentations:edit.back')}
        </PageTitle.BackButton>
        <PageTitle.Header component="div">
          <Stack direction="row" alignItems="center" spacing={1} mb={1}>
            {isEditingName ? (
              <TextField
                value={tempName}
                onChange={(e) => setTempName(e.target.value)}
                onBlur={() => {
                  const trimmed = tempName.trim();
                  if (trimmed && trimmed !== name.trim()) {
                    setName(trimmed);
                  }
                  setIsEditingName(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter') {
                    const trimmed = tempName.trim();
                    if (trimmed && trimmed !== name.trim()) {
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
                  aria-label="Edit presentation definition name"
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
                fullWidth
                multiline
                rows={2}
                value={tempDescription}
                onChange={(e) => setTempDescription(e.target.value)}
                onBlur={() => {
                  const trimmed = tempDescription.trim();
                  if (trimmed !== description.trim()) {
                    setDescription(trimmed);
                  }
                  setIsEditingDescription(false);
                }}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && e.ctrlKey) {
                    const trimmed = tempDescription.trim();
                    if (trimmed !== description.trim()) {
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
                  aria-label="Edit presentation definition description"
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

      <VerifiablePresentationForm
        initial={data}
        name={name}
        description={description}
        onNameChange={setName}
        onDescriptionChange={setDescription}
        submitting={updateVP.isPending}
        submitLabel={t('common:actions.save')}
        onSubmit={handleSubmit}
        onDelete={(): void => setDeleteOpen(true)}
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
