// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {useConfig} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateConnection from '../api/useCreateConnection';
import ConnectionCreateHint from '../components/ConnectionCreateHint';
import ConnectionForm from '../components/ConnectionForm';
import ConnectionNameStep from '../components/create-connection/ConnectionNameStep';
import SelectConnectionType, {
  type SelectableConnectionType,
} from '../components/create-connection/SelectConnectionType';
import TrustedIssuerCreateForm from '../components/TrustedIssuerCreateForm';
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

/**
 * Three-step full-screen wizard for adding a custom connection: pick the type, name it, then
 * enter the credentials/endpoints and create it. The `'trusted-idp'` type renders the dedicated
 * trusted-issuer form instead of the generic configure step.
 */
export default function ConnectionCreateWizardPage(): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const routes = useConnectionRoutes();
  const {getGateCallbackUrl} = useConfig();

  const [step, setStep] = useState<Step>(Step.TYPE);
  const [selectedType, setSelectedType] = useState<SelectableConnectionType | null>(null);
  const [connectionName, setConnectionName] = useState('');
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [nameError, setNameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);

  const isTrustedIdp: boolean = selectedType === 'trusted-idp';

  // Defaults to OIDC before the user picks a type on the first step; the trusted-idp pseudo-type
  // renders via TrustedIssuerCreateForm instead, so this is only read when rendering the generic
  // configure step.
  const activeType: ConnectionType =
    selectedType && selectedType !== 'trusted-idp' ? selectedType : ConnectionTypes.OIDC;
  const createMutation = useCreateConnection(activeType);
  const meta = VENDOR_META_BY_TYPE[activeType];
  const fields = CONNECTION_FORM_FIELDS[activeType];
  const createFields = useMemo(() => fieldsForMode(activeType, 'create'), [activeType]);
  const redirectUri = getGateCallbackUrl();
  const emptyValues = useMemo(() => emptyFormValues(fields, redirectUri), [fields, redirectUri]);

  // Only federated login providers carry a redirect URI to register with the provider.
  const usesRedirectUri: boolean = fields.some((field) => field.name === 'redirectUri');

  const trimmedName: string = connectionName.trim();
  const values: ConnectionFormValues = {...emptyValues, ...editedValues, name: trimmedName};
  const formValid: boolean = Object.keys(validateConnectionForm(values, createFields, 'create')).length === 0;

  const close = (): void => {
    void navigate(routes.connections.list());
  };

  const progress: number = ((ALL_STEPS.indexOf(step) + 1) / ALL_STEPS.length) * 100;

  const bounceToNameStep = (): void => {
    setNameError(t('error.duplicateName', 'A connection with this name already exists.'));
    setStep(Step.NAME);
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
    setNameError(null);
    setGeneralError(null);
    const payload = formValuesToRequest(values, fields, {mode: 'create', secretReplaced: true});
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => void navigate(routes.connections.detail(activeType, created.id)),
      onError: (error: Error) => {
        if (isConflictError(error)) {
          bounceToNameStep();
        } else {
          setGeneralError(getErrorMessage(error, t, 'create.error', 'Failed to create connection.'));
        }
      },
    });
  };

  const crumbs = [
    {key: 'connections', label: t('listing.title'), onClick: close},
    {key: 'add', label: t('wizard.title'), onClick: () => setStep(Step.TYPE)},
    ...(step === Step.TYPE ? [{key: 'type', label: t('wizard.steps.type')}] : []),
    ...(step === Step.NAME ? [{key: 'name', label: t('wizard.steps.name', 'Details')}] : []),
    ...(step === Step.CONFIGURE ? [{key: 'configure', label: t('form.chrome.configure')}] : []),
  ];

  const footer: JSX.Element | null = (() => {
    // Picking a type card is itself the step's action, so the type step carries no footer.
    if (step === Step.TYPE) {
      return null;
    }
    if (step === Step.NAME) {
      return (
        <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Button variant="outlined" onClick={() => setStep(Step.TYPE)} sx={{minWidth: 100}}>
            {t('common:actions.back', 'Back')}
          </Button>
          <Button
            variant="contained"
            disabled={!trimmedName}
            onClick={() => setStep(Step.CONFIGURE)}
            data-testid="wizard-continue"
          >
            {t('common:actions.continue', 'Continue')}
          </Button>
        </Box>
      );
    }
    // The trusted-idp step renders its own Back + submit footer (see TrustedIssuerCreateForm).
    if (isTrustedIdp) {
      return null;
    }
    return (
      <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
        <Button variant="outlined" onClick={() => setStep(Step.NAME)} sx={{minWidth: 100}}>
          {t('common:actions.back', 'Back')}
        </Button>
        <Button
          variant="contained"
          disabled={!formValid || createMutation.isPending}
          onClick={handleCreate}
          data-testid="wizard-create"
        >
          {t('form.actions.create', 'Create connection')}
        </Button>
      </Box>
    );
  })();

  return (
    <FullScreenCreationWizardLayout onClose={close} progress={progress} breadcrumbItems={crumbs} footer={footer}>
      {step === Step.TYPE && (
        <SelectConnectionType
          selectedType={selectedType}
          onSelect={(type) => {
            setSelectedType(type);
            setStep(Step.NAME);
          }}
        />
      )}

      {step === Step.NAME && (
        <ConnectionNameStep
          name={connectionName}
          onNameChange={(name) => {
            setConnectionName(name);
            setNameError(null);
            setGeneralError(null);
          }}
          nameError={nameError}
        />
      )}

      {step === Step.CONFIGURE && isTrustedIdp && (
        <TrustedIssuerCreateForm
          name={trimmedName}
          onNameConflict={bounceToNameStep}
          onBack={() => setStep(Step.NAME)}
        />
      )}

      {step === Step.CONFIGURE && !isTrustedIdp && (
        <Stack direction="column" spacing={3}>
          <Stack direction="column" spacing={1}>
            <Typography variant="h1" gutterBottom>
              {t('wizard.configure.heading')}
            </Typography>
            <Typography variant="subtitle1" gutterBottom>
              {t('wizard.configure.subheading')}
            </Typography>
          </Stack>

          {usesRedirectUri && (
            <ConnectionCreateHint
              instruction={t(
                'wizard.configure.redirectHint',
                'Register the redirect URI below with your identity provider as an allowed callback URL, then enter the credentials and endpoints it gives you.',
              )}
              redirectUri={redirectUri}
            />
          )}

          <Paper variant="outlined" sx={{p: 3}}>
            <ConnectionForm
              type={activeType}
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
            <Alert severity="error" onClose={clearCreateError}>
              {generalError}
            </Alert>
          )}
        </Stack>
      )}
    </FullScreenCreationWizardLayout>
  );
}
