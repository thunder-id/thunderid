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
import {ChevronLeft} from '@wso2/oxygen-ui-icons-react';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateConnection from '../api/useCreateConnection';
import ConnectionForm, {type ConnectionFormSnapshot} from '../components/ConnectionForm';
import ConnectionFullPageLayout from '../components/ConnectionFullPageLayout';
import ConnectionAttributeMappingStep from '../components/create-connection/ConnectionAttributeMappingStep';
import SelectConnectionType from '../components/create-connection/SelectConnectionType';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import {type AttributeConfiguration, type ConnectionType, ConnectionTypes} from '../models/connection';
import {emptyFormValues, formValuesToRequest} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

const Step = {TYPE: 'TYPE', CONFIGURE: 'CONFIGURE', ATTRIBUTES: 'ATTRIBUTES'} as const;
type Step = (typeof Step)[keyof typeof Step];
const ALL_STEPS: Step[] = [Step.TYPE, Step.CONFIGURE, Step.ATTRIBUTES];

/**
 * Three-step full-screen wizard for adding a custom connection: pick the type, enter the
 * credentials/endpoints (with a connection name), then the optional attribute mapping. Only
 * Custom OIDC is wired today; the wizard always configures the oidc type.
 */
export default function ConnectionCreateWizardPage(): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {getServerUrl} = useConfig();

  const [step, setStep] = useState<Step>(Step.TYPE);
  const [selectedType, setSelectedType] = useState<ConnectionType | null>(null);
  const [snapshot, setSnapshot] = useState<ConnectionFormSnapshot | null>(null);
  const [attrConfig, setAttrConfig] = useState<AttributeConfiguration | undefined>(undefined);
  const [attrValid, setAttrValid] = useState(true);
  const [nameError, setNameError] = useState<string | null>(null);

  // Only Custom OIDC is wired today; the wizard always configures the oidc type.
  const createMutation = useCreateConnection(ConnectionTypes.OIDC);
  const meta = VENDOR_META_BY_TYPE[ConnectionTypes.OIDC];
  const fields = CONNECTION_FORM_FIELDS[ConnectionTypes.OIDC];
  const redirectUri = `${getServerUrl()}/oauth2/callback`;
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

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

  const crumbs = [
    {key: 'connections', label: t('listing.title'), onClick: close},
    {key: 'add', label: t('wizard.title'), onClick: () => setStep(Step.TYPE)},
    ...(step === Step.TYPE ? [{key: 'type', label: t('wizard.steps.type')}] : []),
    ...(step === Step.CONFIGURE ? [{key: 'configure', label: t('form.chrome.configure')}] : []),
    ...(step === Step.ATTRIBUTES
      ? [
          {key: 'configure', label: t('form.chrome.configure'), onClick: () => setStep(Step.CONFIGURE)},
          {key: 'attributes', label: t('wizard.steps.attributeMapping')},
        ]
      : []),
  ];

  return (
    <ConnectionFullPageLayout
      label={t('wizard.title')}
      onClose={close}
      progress={progress}
      breadcrumb={<AppBreadcrumbs items={crumbs} />}
    >
      {step === Step.TYPE && (
        <>
          <SelectConnectionType selectedType={selectedType} onSelect={setSelectedType} />
          <Box sx={{mt: 4, display: 'flex', justifyContent: 'flex-end'}}>
            <Button
              variant="contained"
              disabled={!selectedType}
              onClick={() => setStep(Step.CONFIGURE)}
              data-testid="wizard-continue"
            >
              {t('common:actions.continue')}
            </Button>
          </Box>
        </>
      )}

      {step === Step.CONFIGURE && (
        <Stack direction="column" spacing={3}>
          <Stack direction="column" spacing={1}>
            <Typography variant="h4" fontWeight={700}>
              {t('wizard.configure.heading')}
            </Typography>
            <Typography variant="body1" color="text.secondary">
              {t('wizard.configure.subheading')}
            </Typography>
          </Stack>

          <Paper variant="outlined" sx={{p: 3}}>
            <ConnectionForm
              type={ConnectionTypes.OIDC}
              mode="create"
              initialValues={emptyValues}
              hasStoredSecret={false}
              vendorDisplayName={meta.displayName}
              nameError={nameError}
              onChange={setSnapshot}
            />
          </Paper>

          <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.TYPE)}>
              {t('common:actions.back')}
            </Button>
            <Button
              variant="contained"
              disabled={!snapshot?.valid}
              onClick={() => setStep(Step.ATTRIBUTES)}
              data-testid="wizard-configure-continue"
            >
              {t('common:actions.continue')}
            </Button>
          </Box>
        </Stack>
      )}

      {step === Step.ATTRIBUTES && (
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
