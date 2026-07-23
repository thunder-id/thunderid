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
import ConnectionCreateHint from '../components/ConnectionCreateHint';
import ConnectionForm from '../components/ConnectionForm';
import ConnectionFullPageLayout from '../components/ConnectionFullPageLayout';
import ConnectionNameStep from '../components/create-connection/ConnectionNameStep';
import SelectConnectionType, {
  type SelectableConnectionType,
} from '../components/create-connection/SelectConnectionType';
import {CONNECTION_FORM_FIELDS, fieldsForMode} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import {type ConnectionResponse, type ConnectionType, ConnectionTypes} from '../models/connection';
import {
  type ConnectionFormValues,
  emptyFormValues,
  formValuesToRequest,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';

const Step = {TYPE: 'TYPE', NAME: 'NAME', CONFIGURE: 'CONFIGURE'} as const;
type Step = (typeof Step)[keyof typeof Step];
const ALL_STEPS: Step[] = [Step.TYPE, Step.NAME, Step.CONFIGURE];

/** Props passed to a custom configure step (see {@link ConnectionCreateWizardPageProps.customConfigureSteps}). */
export interface CustomConfigureStepProps {
  /** Connection name collected on the wizard's name step. */
  name: string;
  /** Call when the create request 409s on a duplicate name, to bounce back to the name step. */
  onNameConflict: () => void;
}

interface ConnectionCreateWizardPageProps {
  /**
   * Renders a fully custom configure step (step 3) for the given selectable-type key instead of
   * the generic `ConnectionForm` + create button — e.g. a UI-only pseudo-type like
   * `'trusted-idp'` that has no backend /connections vendor route of its own. The supplied
   * render function receives the name collected on the wizard's name step; the wizard still
   * provides the chrome (breadcrumb, progress, Back, and the X close button).
   */
  customConfigureSteps?: Record<string, (props: CustomConfigureStepProps) => ReactNode>;
}

/**
 * Three-step full-screen wizard for adding a custom connection: pick the type, name it, then
 * enter the credentials/endpoints and create it. A type can opt out of the generic configure step
 * via `customConfigureSteps`.
 */
export default function ConnectionCreateWizardPage({
  customConfigureSteps = undefined,
}: ConnectionCreateWizardPageProps): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const routes = useConnectionRoutes();
  const {getGateCallbackUrl} = useConfig();

  const [step, setStep] = useState<Step>(Step.TYPE);
  const [selectedType, setSelectedType] = useState<SelectableConnectionType | null>(null);
  const [connectionName, setConnectionName] = useState('');
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [nameError, setNameError] = useState<string | null>(null);

  const customStepRenderer = selectedType ? customConfigureSteps?.[selectedType] : undefined;
  const customTypes: string[] = useMemo(() => Object.keys(customConfigureSteps ?? {}), [customConfigureSteps]);

  // Defaults to OIDC before the user picks a type on the first step; the SMS placeholder is
  // disabled and unselectable, and pseudo-types render via `customStepRenderer` instead, so this
  // is only read when rendering the generic configure step.
  const activeType: ConnectionType =
    selectedType && selectedType !== 'trusted-idp' ? selectedType : ConnectionTypes.OIDC;
  const createMutation = useCreateConnection(activeType);
  const meta = VENDOR_META_BY_TYPE[activeType];
  const fields = CONNECTION_FORM_FIELDS[activeType];
  const createFields = useMemo(() => fieldsForMode(activeType, 'create'), [activeType]);
  const redirectUri = getGateCallbackUrl();
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  const trimmedName: string = connectionName.trim();
  const values: ConnectionFormValues = {...emptyValues, ...editedValues, name: trimmedName};
  const formValid: boolean = Object.keys(validateConnectionForm(values, createFields, 'create')).length === 0;

  const close = (): void => {
    void navigate(routes.connections.list());
  };

  const progress: number = ((ALL_STEPS.indexOf(step) + 1) / ALL_STEPS.length) * 100;

  const bounceToNameStep = (): void => {
    setNameError(t('error.duplicateName'));
    setStep(Step.NAME);
  };

  const handleCreate = (): void => {
    if (!formValid) {
      return;
    }
    setNameError(null);
    const payload = formValuesToRequest(values, fields, {mode: 'create', secretReplaced: true});
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => void navigate(routes.connections.detail(activeType, created.id)),
      onError: (error: Error) => {
        if (isConflictError(error)) {
          bounceToNameStep();
        }
      },
    });
  };

  const crumbs = [
    {key: 'connections', label: t('listing.title'), onClick: close},
    {key: 'add', label: t('wizard.title'), onClick: () => setStep(Step.TYPE)},
    ...(step === Step.TYPE ? [{key: 'type', label: t('wizard.steps.type')}] : []),
    ...(step === Step.NAME ? [{key: 'name', label: t('wizard.steps.name', 'Name')}] : []),
    ...(step === Step.CONFIGURE ? [{key: 'configure', label: t('form.chrome.configure')}] : []),
  ];

  return (
    <ConnectionFullPageLayout
      label={t('wizard.title')}
      onClose={close}
      progress={progress}
      breadcrumb={<AppBreadcrumbs items={crumbs} />}
      fullWidthContent={step === Step.TYPE}
    >
      {step === Step.TYPE && (
        <>
          <SelectConnectionType selectedType={selectedType} onSelect={setSelectedType} customTypes={customTypes} />
          <Box sx={{mt: 4, display: 'flex', justifyContent: 'flex-end'}}>
            <Button
              variant="contained"
              disabled={!selectedType}
              onClick={() => setStep(Step.NAME)}
              data-testid="wizard-continue"
            >
              {t('common:actions.continue')}
            </Button>
          </Box>
        </>
      )}

      {step === Step.NAME && (
        <Stack direction="column" spacing={3}>
          <ConnectionNameStep
            name={connectionName}
            onNameChange={(name) => {
              setConnectionName(name);
              setNameError(null);
            }}
            nameError={nameError}
          />
          <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.TYPE)}>
              {t('common:actions.back')}
            </Button>
            <Button
              variant="contained"
              disabled={!trimmedName}
              onClick={() => setStep(Step.CONFIGURE)}
              data-testid="wizard-continue"
            >
              {t('common:actions.continue')}
            </Button>
          </Box>
        </Stack>
      )}

      {step === Step.CONFIGURE && customStepRenderer && (
        <Stack direction="column" spacing={3}>
          {customStepRenderer({name: trimmedName, onNameConflict: bounceToNameStep})}

          <Box sx={{display: 'flex', justifyContent: 'flex-start'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.NAME)}>
              {t('common:actions.back')}
            </Button>
          </Box>
        </Stack>
      )}

      {step === Step.CONFIGURE && !customStepRenderer && (
        <Stack direction="column" spacing={3}>
          <Stack direction="column" spacing={1}>
            <Typography variant="h1" gutterBottom>
              {t('wizard.configure.heading')}
            </Typography>
            <Typography variant="subtitle1" gutterBottom>
              {t('wizard.configure.subheading')}
            </Typography>
          </Stack>

          <ConnectionCreateHint
            instruction={t(
              'wizard.configure.redirectHint',
              'Register the redirect URI below with your identity provider as an allowed callback URL, then enter the credentials and endpoints it gives you.',
            )}
            redirectUri={redirectUri}
          />

          <Paper variant="outlined" sx={{p: 3}}>
            <ConnectionForm
              type={activeType}
              mode="create"
              values={values}
              secretReplacing={false}
              hasStoredSecret={false}
              vendorDisplayName={meta.displayName}
              showNameField={false}
              onFieldChange={(name, value) => setEditedValues((prev) => ({...prev, [name]: value}))}
              onSecretReplacingChange={() => undefined}
            />
          </Paper>

          <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
            <Button variant="outlined" startIcon={<ChevronLeft size={16} />} onClick={() => setStep(Step.NAME)}>
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
