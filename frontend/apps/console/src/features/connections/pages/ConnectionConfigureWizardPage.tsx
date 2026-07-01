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
import ConnectionForm, {type ConnectionFormSnapshot} from '../components/ConnectionForm';
import ConnectionFullPageLayout from '../components/ConnectionFullPageLayout';
import ConnectionAttributeMappingStep from '../components/create-connection/ConnectionAttributeMappingStep';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import type {AttributeConfiguration, ConnectionType} from '../models/connection';
import {emptyFormValues, formValuesToRequest} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

const Step = {CONFIGURE: 'CONFIGURE', ATTRIBUTES: 'ATTRIBUTES'} as const;
type Step = (typeof Step)[keyof typeof Step];
const ALL_STEPS: Step[] = [Step.CONFIGURE, Step.ATTRIBUTES];

/**
 * Two-step full-screen wizard for configuring a branded catalog vendor (Google/GitHub):
 * step 1 collects credentials, step 2 the optional attribute mapping. The connection name is
 * fixed to the vendor display name.
 */
export default function ConnectionConfigureWizardPage(): JSX.Element | null {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {getServerUrl} = useConfig();
  const {type} = useParams<{type: string}>();

  const connectionType = type as ConnectionType;
  const meta = VENDOR_META_BY_TYPE[connectionType];

  const createMutation = useCreateConnection(connectionType);

  const [step, setStep] = useState<Step>(Step.CONFIGURE);
  const [snapshot, setSnapshot] = useState<ConnectionFormSnapshot | null>(null);
  const [attrConfig, setAttrConfig] = useState<AttributeConfiguration | undefined>(undefined);
  const [attrValid, setAttrValid] = useState(true);
  const [nameError, setNameError] = useState<string | null>(null);

  useEffect(() => {
    if (!meta) {
      void navigate('/connections');
    }
  }, [meta, navigate]);

  const fields = useMemo(() => (meta ? CONNECTION_FORM_FIELDS[connectionType] : []), [meta, connectionType]);
  const redirectUri = `${getServerUrl()}/oauth2/callback`;
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  if (!meta) {
    return null;
  }

  const close = (): void => {
    void navigate('/connections');
  };

  const progress: number = ((ALL_STEPS.indexOf(step) + 1) / ALL_STEPS.length) * 100;

  const handleCreate = (): void => {
    if (!snapshot?.valid || !attrValid) {
      return;
    }
    setNameError(null);
    const payload = {
      ...formValuesToRequest(snapshot.values, fields, {mode: 'create', secretReplaced: true}),
      name: meta.displayName,
      attributeConfiguration: attrConfig,
    };
    createMutation.mutate(payload, {
      onSuccess: () => close(),
      onError: (error: Error) => {
        if (isConflictError(error)) {
          setNameError(t('error.duplicateName'));
        }
      },
    });
  };

  const crumbs =
    step === Step.CONFIGURE
      ? [
          {key: 'connections', label: t('listing.title'), onClick: close},
          {key: 'vendor', label: meta.displayName},
          {key: 'configure', label: t('form.chrome.configure')},
        ]
      : [
          {key: 'connections', label: t('listing.title'), onClick: close},
          {key: 'vendor', label: meta.displayName},
          {key: 'configure', label: t('form.chrome.configure'), onClick: () => setStep(Step.CONFIGURE)},
          {key: 'attributes', label: t('wizard.steps.attributeMapping')},
        ];

  return (
    <ConnectionFullPageLayout
      label={t('form.chrome.configure')}
      onClose={close}
      progress={progress}
      breadcrumb={<AppBreadcrumbs items={crumbs} />}
    >
      {step === Step.CONFIGURE ? (
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
              initialValues={emptyValues}
              hasStoredSecret={false}
              vendorDisplayName={meta.displayName}
              nameError={nameError}
              showNameField={false}
              onChange={setSnapshot}
            />
          </Paper>

          <Box sx={{display: 'flex', justifyContent: 'flex-end'}}>
            <Button
              variant="contained"
              disabled={!snapshot?.valid}
              onClick={() => setStep(Step.ATTRIBUTES)}
              data-testid="wizard-continue"
            >
              {t('common:actions.continue')}
            </Button>
          </Box>
        </Stack>
      ) : (
        <ConnectionAttributeMappingStep
          vendorDisplayName={meta.displayName}
          onChange={(config, valid) => {
            setAttrConfig(config);
            setAttrValid(valid);
          }}
          onBack={() => setStep(Step.CONFIGURE)}
          onCreate={handleCreate}
          isPending={createMutation.isPending}
          createDisabled={!snapshot?.valid || !attrValid}
        />
      )}
    </ConnectionFullPageLayout>
  );
}
