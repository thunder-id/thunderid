// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useCreateConnection from '../api/useCreateConnection';
import ConnectionCreateHint from '../components/ConnectionCreateHint';
import ConnectionForm from '../components/ConnectionForm';
import {CONNECTION_FORM_FIELDS, fieldsForMode} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import type {ConnectionResponse, ConnectionType} from '../models/connection';
import {
  type ConnectionFormValues,
  emptyFormValues,
  formValuesToRequest,
  validateConnectionForm,
} from '../utils/connectionFormMapping';

/**
 * Full-screen wizard for configuring a branded catalog vendor: a single credentials step. The
 * connection name is fixed to the vendor display name.
 */
export default function ConnectionConfigureWizardPage(): JSX.Element | null {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const routes = useConnectionRoutes();
  const {getGateCallbackUrl} = useConfig();
  const {type} = useParams<{type: string}>();

  const connectionType = type as ConnectionType;
  const meta = VENDOR_META_BY_TYPE[connectionType];

  const createMutation = useCreateConnection(connectionType);

  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [generalError, setGeneralError] = useState<string | null>(null);

  useEffect(() => {
    if (!meta) {
      void navigate(routes.connections.list());
    }
  }, [meta, navigate, routes]);

  const fields = useMemo(() => (meta ? CONNECTION_FORM_FIELDS[connectionType] : []), [meta, connectionType]);
  const redirectUri = getGateCallbackUrl();
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  if (!meta) {
    return null;
  }

  const values: ConnectionFormValues = {...emptyValues, ...editedValues};
  // The connection name is fixed to the vendor display name, so it is hidden and excluded from validation.
  const visibleFields = fieldsForMode(connectionType, 'create').filter((field) => field.name !== 'name');
  const formValid: boolean = Object.keys(validateConnectionForm(values, visibleFields, 'create')).length === 0;

  const close = (): void => {
    void navigate(routes.connections.list());
  };

  // A create failure is stale once the user edits any field. Only reset the mutation once it has
  // actually failed: resetting while it's still pending would flip isPending back to false and
  // re-enable the create button before the in-flight request settles.
  const clearCreateError = (): void => {
    setGeneralError(null);
    if (createMutation.isError) {
      createMutation.reset();
    }
  };

  const handleCreate = (): void => {
    if (!formValid) {
      return;
    }
    setGeneralError(null);
    const payload = {
      ...formValuesToRequest(values, fields, {mode: 'create', secretReplaced: true}),
      name: meta.displayName,
    };
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => void navigate(routes.connections.detail(connectionType, created.id)),
      onError: (error: Error) => {
        // The connection name here is fixed to the vendor display name and is not user-editable
        // (see the visibleFields filter above), so a 409 duplicate-name conflict has no more
        // specific place to go than the same general error surface as any other failure.
        setGeneralError(getErrorMessage(error, t, 'create.error', 'Failed to create connection.'));
      },
    });
  };

  const crumbs = [
    {key: 'connections', label: t('listing.title'), onClick: close},
    {key: 'vendor', label: meta.displayName},
    {key: 'configure', label: t('form.chrome.configure')},
  ];

  return (
    <FullScreenCreationWizardLayout
      onClose={close}
      progress={100}
      breadcrumbItems={crumbs}
      footer={
        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button
            variant="contained"
            disabled={!formValid || createMutation.isPending}
            onClick={handleCreate}
            data-testid="wizard-create"
          >
            {t('form.actions.create', 'Create connection')}
          </Button>
        </Box>
      }
    >
      <Stack direction="column" spacing={3}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h1" gutterBottom>
            {t('configure.heading', {vendor: meta.displayName})}
          </Typography>
          <Typography variant="subtitle1" gutterBottom>
            {t('configure.subheading')}
          </Typography>
        </Stack>

        {meta.createHintKey && (
          <ConnectionCreateHint
            instruction={t(meta.createHintKey, {
              vendor: meta.displayName,
              defaultValue:
                'Create an OAuth client for {{vendor}}, then register the redirect URI below and enter the client ID and client secret it gives you.',
            })}
            redirectUri={redirectUri}
          />
        )}

        <Paper variant="outlined" sx={{p: 3}}>
          <ConnectionForm
            type={connectionType}
            mode="create"
            values={values}
            secretReplacing={false}
            hasStoredSecret={false}
            vendorDisplayName={meta.displayName}
            showNameField={false}
            onFieldChange={(name, value) => {
              clearCreateError();
              setEditedValues((prev) => ({...prev, [name]: value}));
            }}
            onSecretReplacingChange={() => undefined}
          />
        </Paper>

        {generalError && (
          <Alert severity="error" onClose={clearCreateError} data-testid="wizard-create-error">
            {generalError}
          </Alert>
        )}
      </Stack>
    </FullScreenCreationWizardLayout>
  );
}
