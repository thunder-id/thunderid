// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  Alert,
  Box,
  Button,
  CircularProgress,
  FormControl,
  FormLabel,
  PageContent,
  PageTitle,
  Stack,
  TextField,
} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useGetEnvironmentVariable from '../api/useGetEnvironmentVariable';
import useUpdateEnvironmentVariable from '../api/useUpdateEnvironmentVariable';
import EnvironmentVariableDeleteDialog from '../components/EnvironmentVariableDeleteDialog';

/**
 * Page for editing an environment variable's value or description. The key is immutable, because it is
 * the placeholder that configuration references.
 */
export default function EnvironmentVariableEditPage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {envId = '', environmentVariableId = ''} = useParams<{envId: string; environmentVariableId: string}>();

  const {data, isLoading, error} = useGetEnvironmentVariable(envId, environmentVariableId);
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  if (isLoading) {
    return (
      <PageContent>
        <Box sx={{display: 'flex', justifyContent: 'center', py: 8}}>
          <CircularProgress />
        </Box>
      </PageContent>
    );
  }

  if (error) {
    return (
      <PageContent>
        <Alert severity="error">
          {error.message || t('environmentVariables:edit.error', 'Failed to load the environment variable')}
        </Alert>
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{data?.key ?? t('environmentVariables:edit.title', 'Environment Variable')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('environmentVariables:edit.subtitle', 'Update the value applied to your Data Planes')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            color="error"
            onClick={() => {
              setDeleteOpen(true);
            }}
          >
            {t('common:actions.delete', 'Delete')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {data && <EnvironmentVariableForm envId={envId} environmentVariableId={environmentVariableId} initial={data} />}

      <EnvironmentVariableDeleteDialog
        open={deleteOpen}
        environmentVariableId={environmentVariableId}
        onClose={() => {
          setDeleteOpen(false);
          void navigate(`/promotions/${envId}/variables`);
        }}
      />
    </PageContent>
  );
}

/**
 * The edit form, seeded from the loaded variable.
 *
 * It is a separate component so its state initializes from props on mount, once the variable is
 * available. Seeding state from an effect instead would re-render on every load and would risk
 * discarding the user's edits on a background refetch.
 */
function EnvironmentVariableForm({
  envId,
  environmentVariableId,
  initial,
}: {
  envId: string;
  environmentVariableId: string;
  initial: {value: string; description?: string};
}): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const updateEnvironmentVariable = useUpdateEnvironmentVariable(envId);

  const [value, setValue] = useState<string>(initial.value);
  const [description, setDescription] = useState<string>(initial.description ?? '');

  return (
    <Stack spacing={3} sx={{maxWidth: 640}}>
      <FormControl fullWidth>
        <FormLabel>{t('environmentVariables:form.value.label', 'Value')}</FormLabel>
        <TextField
          value={value}
          onChange={(event) => {
            setValue(event.target.value);
          }}
          helperText={t(
            'environmentVariables:form.value.help',
            'For a list, use a JSON array such as ["https://app.example.com/callback"].',
          )}
          fullWidth
        />
      </FormControl>

      <FormControl fullWidth>
        <FormLabel>{t('environmentVariables:form.description.label', 'Description')}</FormLabel>
        <TextField
          value={description}
          onChange={(event) => {
            setDescription(event.target.value);
          }}
          fullWidth
        />
      </FormControl>

      <Stack direction="row" spacing={2} justifyContent="flex-end">
        <Button
          onClick={() => {
            void navigate(`/promotions/${envId}/variables`);
          }}
        >
          {t('common:actions.cancel', 'Cancel')}
        </Button>
        <Button
          variant="contained"
          onClick={() => {
            updateEnvironmentVariable.mutate({id: environmentVariableId, data: {value, description}});
          }}
          disabled={updateEnvironmentVariable.isPending || value.trim() === ''}
        >
          {updateEnvironmentVariable.isPending
            ? t('common:status.saving', 'Saving...')
            : t('common:actions.save', 'Save')}
        </Button>
      </Stack>
    </Stack>
  );
}
