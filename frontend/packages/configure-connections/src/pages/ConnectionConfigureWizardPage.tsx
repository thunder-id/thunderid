/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useConfig} from '@thunderid/contexts';
import {AppBreadcrumbs, Box, Button, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX, useEffect, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useCreateConnection from '../api/useCreateConnection';
import ConnectionForm from '../components/ConnectionForm';
import ConnectionFullPageLayout from '../components/ConnectionFullPageLayout';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import type {ConnectionResponse, ConnectionType} from '../models/connection';
import {
  type ConnectionFormValues,
  emptyFormValues,
  formValuesToRequest,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

/**
 * Full-screen wizard for configuring a branded catalog vendor: a single credentials step. The
 * connection name is fixed to the vendor display name.
 */
export default function ConnectionConfigureWizardPage(): JSX.Element | null {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {getGateCallbackUrl} = useConfig();
  const {type} = useParams<{type: string}>();

  const connectionType = type as ConnectionType;
  const meta = VENDOR_META_BY_TYPE[connectionType];

  const createMutation = useCreateConnection(connectionType);

  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [nameError, setNameError] = useState<string | null>(null);

  useEffect(() => {
    if (!meta) {
      void navigate('/connections');
    }
  }, [meta, navigate]);

  const fields = useMemo(() => (meta ? CONNECTION_FORM_FIELDS[connectionType] : []), [meta, connectionType]);
  const redirectUri = getGateCallbackUrl();
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  if (!meta) {
    return null;
  }

  const values: ConnectionFormValues = {...emptyValues, ...editedValues};
  // The connection name is fixed to the vendor display name, so it is hidden and excluded from validation.
  const visibleFields = fields.filter((field) => field.name !== 'name');
  const formValid: boolean = Object.keys(validateConnectionForm(values, visibleFields, 'create')).length === 0;

  const close = (): void => {
    void navigate('/connections');
  };

  const handleCreate = (): void => {
    if (!formValid) {
      return;
    }
    setNameError(null);
    const payload = {
      ...formValuesToRequest(values, fields, {mode: 'create', secretReplaced: true}),
      name: meta.displayName,
    };
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => void navigate(`/connections/${connectionType}/${created.id}`),
      onError: (error: Error) => {
        if (isConflictError(error)) {
          setNameError(t('error.duplicateName'));
        }
      },
    });
  };

  const crumbs = [
    {key: 'connections', label: t('listing.title'), onClick: close},
    {key: 'vendor', label: meta.displayName},
    {key: 'configure', label: t('form.chrome.configure')},
  ];

  return (
    <ConnectionFullPageLayout
      label={t('form.chrome.configure')}
      onClose={close}
      breadcrumb={<AppBreadcrumbs items={crumbs} />}
    >
      <Stack direction="column" spacing={3}>
        <Stack direction="column" spacing={1}>
          <Typography variant="h4" fontWeight={700}>
            {t('configure.heading', {vendor: meta.displayName})}
          </Typography>
          <Typography variant="body1" color="text.secondary">
            {t('configure.subheading')}
          </Typography>
        </Stack>

        <Paper variant="outlined" sx={{p: 3}}>
          <ConnectionForm
            type={connectionType}
            mode="create"
            values={values}
            secretReplacing={false}
            hasStoredSecret={false}
            vendorDisplayName={meta.displayName}
            nameError={nameError}
            showNameField={false}
            onFieldChange={(name, value) => setEditedValues((prev) => ({...prev, [name]: value}))}
            onSecretReplacingChange={() => undefined}
          />
        </Paper>

        <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
          <Button
            variant="contained"
            disabled={!formValid || createMutation.isPending}
            onClick={handleCreate}
            data-testid="wizard-create"
          >
            {t('form.actions.create')}
          </Button>
        </Box>
      </Stack>
    </ConnectionFullPageLayout>
  );
}
