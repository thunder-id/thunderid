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
import {type JSX, type ReactNode, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateConnection from '../api/useCreateConnection';
import ConnectionForm from '../components/ConnectionForm';
import ConnectionFullPageLayout from '../components/ConnectionFullPageLayout';
import SelectConnectionType, {
  type SelectableConnectionType,
} from '../components/create-connection/SelectConnectionType';
import {CONNECTION_FORM_FIELDS} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import {type ConnectionResponse, type ConnectionType, ConnectionTypes} from '../models/connection';
import {
  type ConnectionFormValues,
  emptyFormValues,
  formValuesToRequest,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

const Step = {TYPE: 'TYPE', CONFIGURE: 'CONFIGURE'} as const;
type Step = (typeof Step)[keyof typeof Step];
const ALL_STEPS: Step[] = [Step.TYPE, Step.CONFIGURE];

interface ConnectionCreateWizardPageProps {
  /**
   * Renders a fully custom configure step (step 2) for the given selectable-type key instead of
   * the generic `ConnectionForm` + create button — e.g. a UI-only pseudo-type like
   * `'trusted-idp'` that has no backend /connections vendor route of its own. The supplied node
   * owns its own submit action; the wizard still provides the chrome (breadcrumb, progress,
   * Back, and the X close button).
   */
  customConfigureSteps?: Record<string, ReactNode>;
}

/**
 * Two-step full-screen wizard for adding a custom connection: pick the type, then enter the
 * credentials/endpoints (with a connection name) and create it. A type can opt out of the
 * generic configure step via `customConfigureSteps`.
 */
export default function ConnectionCreateWizardPage({
  customConfigureSteps = undefined,
}: ConnectionCreateWizardPageProps): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {getGateCallbackUrl} = useConfig();

  const [step, setStep] = useState<Step>(Step.TYPE);
  const [selectedType, setSelectedType] = useState<SelectableConnectionType | null>(null);
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [nameError, setNameError] = useState<string | null>(null);

  const customStep: ReactNode | undefined = selectedType ? customConfigureSteps?.[selectedType] : undefined;
  const customTypes: string[] = useMemo(() => Object.keys(customConfigureSteps ?? {}), [customConfigureSteps]);

  // Defaults to OIDC before the user picks a type on the first step; the SMS placeholder is
  // disabled and unselectable, and pseudo-types render via `customStep` instead, so this is only
  // read when rendering the generic configure step.
  const activeType: ConnectionType =
    selectedType && selectedType !== 'trusted-idp' ? selectedType : ConnectionTypes.OIDC;
  const createMutation = useCreateConnection(activeType);
  const meta = VENDOR_META_BY_TYPE[activeType];
  const fields = CONNECTION_FORM_FIELDS[activeType];
  const redirectUri = getGateCallbackUrl();
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  const values: ConnectionFormValues = {...emptyValues, ...editedValues};
  const formValid: boolean = Object.keys(validateConnectionForm(values, fields, 'create')).length === 0;

  const close = (): void => {
    void navigate('/connections');
  };

  const progress: number = ((ALL_STEPS.indexOf(step) + 1) / ALL_STEPS.length) * 100;

  const handleCreate = (): void => {
    if (!formValid) {
      return;
    }
    setNameError(null);
    const payload = formValuesToRequest(values, fields, {mode: 'create', secretReplaced: true});
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => void navigate(`/connections/${activeType}/${created.id}`),
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
          <SelectConnectionType selectedType={selectedType} onSelect={setSelectedType} customTypes={customTypes} />
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

      {step === Step.CONFIGURE && customStep && (
        <Stack direction="column" spacing={3}>
          {customStep}

          <Box sx={{display: 'flex', justifyContent: 'flex-start'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.TYPE)}>
              {t('common:actions.back')}
            </Button>
          </Box>
        </Stack>
      )}

      {step === Step.CONFIGURE && !customStep && (
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
              type={activeType}
              mode="create"
              values={values}
              secretReplacing={false}
              hasStoredSecret={false}
              vendorDisplayName={meta.displayName}
              nameError={nameError}
              onFieldChange={(name, value) => setEditedValues((prev) => ({...prev, [name]: value}))}
              onSecretReplacingChange={() => undefined}
            />
          </Paper>

          <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.TYPE)}>
              {t('common:actions.back')}
            </Button>
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
      )}
    </ConnectionFullPageLayout>
  );
}
