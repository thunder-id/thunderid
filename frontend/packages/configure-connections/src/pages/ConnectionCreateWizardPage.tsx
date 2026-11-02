// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, FullScreenCreationWizardLayout} from '@thunderid/components';
import {useConfig, useToast} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress, Paper, Stack, Typography} from '@wso2/oxygen-ui';
import {type JSX, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useConnectionMeta from '../api/useConnectionMeta';
import useCreateConnection from '../api/useCreateConnection';
import useEmailProviders from '../api/useEmailProviders';
import useGetNotificationConfig from '../api/useGetNotificationConfig';
import useSetDefaultEmailSender from '../api/useSetDefaultEmailSender';
import ConnectionCreateHint from '../components/ConnectionCreateHint';
import ConnectionForm from '../components/ConnectionForm';
import ConnectionNameStep from '../components/create-connection/ConnectionNameStep';
import ConnectionOuPickerStep from '../components/create-connection/ConnectionOuPickerStep';
import SelectConnectionType, {
  type SelectableConnectionType,
} from '../components/create-connection/SelectConnectionType';
import SelectPDPProtocol from '../components/create-connection/SelectPDPProtocol';
import TrustedIssuerCreateForm from '../components/TrustedIssuerCreateForm';
import {CONNECTION_FORM_FIELDS, fieldsForMode} from '../config/connectionFormFields';
import {VENDOR_META_BY_TYPE} from '../config/connectionVendorMeta';
import {POLICY_DECISION_POINT_TYPE} from '../constants/connection-wizard';
import useConnectionOuSelection from '../hooks/useConnectionOuSelection';
import useConnectionRoutes from '../hooks/useConnectionRoutes';
import {
  type ConnectionResponse,
  type ConnectionType,
  ConnectionTypes,
  type CreateConnectionRequest,
  type OutboundAuthMethod,
} from '../models/connection';
import {
  AUTH_ADDITIONAL_PROPERTIES_FIELD,
  AUTH_TYPE_FIELD,
  authenticationAdditionalPropertiesError,
  authPropertyField,
  type ConnectionFormValues,
  emptyFormValues,
  formValuesToRequest,
  validateConnectionForm,
} from '../utils/connectionFormMapping';
import isConflictError from '../utils/isConflictError';
import {decodeKeyValuePairs} from '../utils/keyValuePairs';

const Step = {TYPE: 'TYPE', PROTOCOL: 'PROTOCOL', NAME: 'NAME', CONFIGURE: 'CONFIGURE'} as const;
type Step = (typeof Step)[keyof typeof Step];

/**
 * Full-screen wizard for adding a custom connection. The `'trusted-idp'` type renders the
 * dedicated trusted-issuer form instead of the generic configure step.
 */
export default function ConnectionCreateWizardPage(): JSX.Element {
  const {t} = useTranslation('connections');
  const navigate = useNavigate();
  const {showToast} = useToast();
  const routes = useConnectionRoutes();
  const {getGateCallbackUrl} = useConfig();

  const [step, setStep] = useState<Step>(Step.TYPE);
  const [selectedType, setSelectedType] = useState<SelectableConnectionType | null>(null);
  const [connectionName, setConnectionName] = useState('');
  const [editedValues, setEditedValues] = useState<ConnectionFormValues>({});
  const [nameError, setNameError] = useState<string | null>(null);
  const [generalError, setGeneralError] = useState<string | null>(null);

  const {ouId, setOuId, needsPicker, isLoading: isOuLoading} = useConnectionOuSelection();
  const [isOuConfirmed, setIsOuConfirmed] = useState(false);

  const isTrustedIdp: boolean = selectedType === 'trusted-idp';
  const isAuthZENPDP: boolean = selectedType === ConnectionTypes.AUTHZEN_PDP;
  const isPDPFlow: boolean = selectedType === POLICY_DECISION_POINT_TYPE || isAuthZENPDP;

  // Defaults to OIDC before the user picks a type on the first step; the trusted-idp pseudo-type
  // renders via TrustedIssuerCreateForm instead, so this is only read when rendering the generic
  // configure step.
  const activeType: ConnectionType =
    selectedType && selectedType !== 'trusted-idp' && selectedType !== POLICY_DECISION_POINT_TYPE
      ? selectedType
      : ConnectionTypes.OIDC;
  const createMutation = useCreateConnection(activeType);
  const meta = VENDOR_META_BY_TYPE[activeType];
  const isEmailSender: boolean = meta?.categories.includes('email') ?? false;
  const emailProvidersQuery = useEmailProviders({enabled: isEmailSender});
  const notificationConfigQuery = useGetNotificationConfig({enabled: isEmailSender});
  const setDefaultSender = useSetDefaultEmailSender();
  // The same query the configure step's form reads, so only the selected method's properties are sent.
  const {data: connectionMeta} = useConnectionMeta(step === Step.CONFIGURE ? activeType : undefined);
  const authenticationMethods: OutboundAuthMethod[] = useMemo(
    () => connectionMeta?.authentication.methods ?? [],
    [connectionMeta],
  );
  const fields = CONNECTION_FORM_FIELDS[activeType];
  const createFields = useMemo(() => fieldsForMode(activeType, 'create'), [activeType]);
  const redirectUri = getGateCallbackUrl();

  // Only federated login providers carry a redirect URI to register with the provider.
  const usesRedirectUri: boolean = fields.some((field) => field.name === 'redirectUri');
  const emptyValues = useMemo(
    () => ({
      ...emptyFormValues(createFields, redirectUri),
      ...(usesRedirectUri ? {redirectUri} : {}),
    }),
    [createFields, redirectUri, usesRedirectUri],
  );

  const trimmedName: string = connectionName.trim();
  const values: ConnectionFormValues = {...emptyValues, ...editedValues, name: trimmedName};
  const formErrors: Record<string, string> = validateConnectionForm(values, createFields, 'create');
  const selectedAuthenticationMethod: OutboundAuthMethod | undefined = authenticationMethods.find(
    (method) => method.type === values[AUTH_TYPE_FIELD],
  );
  const additionalPropertiesMissing: boolean =
    Boolean(selectedAuthenticationMethod?.additionalProperties?.required) &&
    decodeKeyValuePairs(values[AUTH_ADDITIONAL_PROPERTIES_FIELD] ?? '', 'pairs').length === 0;
  const requiredAuthenticationFieldMissing: boolean = (selectedAuthenticationMethod?.fields ?? []).some(
    (field) => field.required && (values[authPropertyField(field.key)] ?? '').trim() === '',
  );
  const formValid: boolean =
    Object.keys(formErrors).length === 0 &&
    !additionalPropertiesMissing &&
    !requiredAuthenticationFieldMissing &&
    !authenticationAdditionalPropertiesError(values, authenticationMethods);
  // The name is entered on its own step, so a format rule on it is reported there.
  const nameFormatError: string | null = trimmedName && formErrors['name'] ? t(formErrors['name']) : null;

  const close = (): void => {
    void navigate(routes.connections.list());
  };

  const activeSteps: Step[] = isPDPFlow
    ? [Step.TYPE, Step.PROTOCOL, Step.NAME, Step.CONFIGURE]
    : [Step.TYPE, Step.NAME, Step.CONFIGURE];
  const progress: number = ((activeSteps.indexOf(step) + 1) / activeSteps.length) * 100;

  const bounceToNameStep = (): void => {
    setNameError(t('error.duplicateName', 'A connection with this name already exists.'));
    setStep(Step.NAME);
  };

  const returnToTypeStep = (): void => {
    if (isPDPFlow) {
      setSelectedType(POLICY_DECISION_POINT_TYPE);
    }
    setStep(Step.TYPE);
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
    if (!ouId) {
      setGeneralError(t('wizard.organizationUnit.required', 'Organization unit is required'));
      return;
    }
    setGeneralError(null);
    const payload: CreateConnectionRequest = {
      ...formValuesToRequest(values, fields, {
        mode: 'create',
        secretReplaced: true,
        authenticationMethods,
      }),
      ouId,
    };
    // The first email sender becomes the default, unless the deployment configuration fixes it.
    const makeDefault: boolean =
      isEmailSender &&
      emailProvidersQuery.isSuccess &&
      emailProvidersQuery.data.length === 0 &&
      notificationConfigQuery.isSuccess &&
      !notificationConfigQuery.data.readOnly?.defaultEmailSenderId;
    createMutation.mutate(payload, {
      onSuccess: (created: ConnectionResponse) => {
        const goToCreated = (): void => void navigate(routes.connections.detail(activeType, created.id));
        if (!makeDefault) {
          goToCreated();
          return;
        }
        // The connection exists by now, so a failure here must not read as a create failure.
        setDefaultSender.mutate(
          {defaultEmailSenderId: created.id},
          {
            onSuccess: goToCreated,
            onError: () => {
              showToast(
                t('create.setDefaultError', 'Connection created, but it could not be made the default email sender.'),
                'warning',
              );
              goToCreated();
            },
          },
        );
      },
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
    {key: 'add', label: t('wizard.title'), onClick: returnToTypeStep},
    ...(step === Step.PROTOCOL || isAuthZENPDP
      ? [
          {
            key: 'pdp',
            label: t('wizard.type.policyDecisionPoint.label', 'Policy decision point (PDP)'),
            onClick: () => {
              setSelectedType(POLICY_DECISION_POINT_TYPE);
              setStep(Step.PROTOCOL);
            },
          },
        ]
      : []),
    ...(isAuthZENPDP
      ? [{key: 'authzen', label: t('wizard.protocol.authzen.label', 'AuthZEN'), onClick: () => setStep(Step.PROTOCOL)}]
      : []),
    ...(step === Step.TYPE ? [{key: 'type', label: t('wizard.steps.type')}] : []),
    ...(step === Step.NAME ? [{key: 'name', label: t('wizard.steps.name', 'Details')}] : []),
    ...(step === Step.CONFIGURE
      ? [
          {
            key: 'configure',
            label: isAuthZENPDP
              ? t('detail.configuration.title', 'Connection configuration')
              : t('form.chrome.configure', 'Configure connection'),
          },
        ]
      : []),
  ];

  const footer: JSX.Element | null = (() => {
    // Picking a type card is itself the step's action, so the type step carries no footer.
    if (step === Step.TYPE) {
      return null;
    }
    if (step === Step.PROTOCOL) {
      return (
        <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Button variant="outlined" onClick={returnToTypeStep} sx={{minWidth: 100}}>
            {t('common:actions.back', 'Back')}
          </Button>
          <Button
            variant="contained"
            disabled={!isAuthZENPDP}
            onClick={() => setStep(Step.NAME)}
            data-testid="wizard-continue"
          >
            {t('common:actions.continue', 'Continue')}
          </Button>
        </Box>
      );
    }
    if (step === Step.NAME) {
      return (
        <Box sx={{display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Button
            variant="outlined"
            onClick={() => setStep(isPDPFlow ? Step.PROTOCOL : Step.TYPE)}
            sx={{minWidth: 100}}
          >
            {t('common:actions.back', 'Back')}
          </Button>
          <Button
            variant="contained"
            disabled={!trimmedName || Boolean(nameFormatError)}
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
          disabled={!formValid || createMutation.isPending || createMutation.isSuccess || setDefaultSender.isPending}
          onClick={handleCreate}
          data-testid="wizard-create"
        >
          {t('form.actions.create', 'Create connection')}
        </Button>
      </Box>
    );
  })();

  if (isOuLoading) {
    return (
      <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
        <CircularProgress aria-label={t('common:status.loading', 'Loading...')} />
      </Box>
    );
  }

  if (needsPicker && !isOuConfirmed) {
    return (
      <ConnectionOuPickerStep
        title={t('wizard.title', 'Add custom connection')}
        value={ouId}
        onChange={setOuId}
        onBack={close}
        onContinue={() => setIsOuConfirmed(true)}
      />
    );
  }

  return (
    <FullScreenCreationWizardLayout onClose={close} progress={progress} breadcrumbItems={crumbs} footer={footer}>
      <DocumentTitle title={t('wizard.title', 'Add custom connection')} />
      {step === Step.TYPE && (
        <SelectConnectionType
          selectedType={selectedType}
          onSelect={(type) => {
            setSelectedType(type);
            setStep(type === POLICY_DECISION_POINT_TYPE ? Step.PROTOCOL : Step.NAME);
          }}
        />
      )}

      {step === Step.PROTOCOL && (
        <SelectPDPProtocol
          selectedProtocol={isAuthZENPDP ? ConnectionTypes.AUTHZEN_PDP : null}
          onSelect={setSelectedType}
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
          nameError={nameError ?? nameFormatError}
        />
      )}

      {step === Step.CONFIGURE && isTrustedIdp && (
        <TrustedIssuerCreateForm
          name={trimmedName}
          ouId={ouId}
          onNameConflict={bounceToNameStep}
          onBack={() => setStep(Step.NAME)}
        />
      )}

      {step === Step.CONFIGURE && !isTrustedIdp && (
        <Stack direction="column" spacing={3}>
          <Stack direction="column" spacing={1}>
            <Typography variant="h1" gutterBottom>
              {isAuthZENPDP
                ? t('detail.configuration.title', 'Connection configuration')
                : t('wizard.configure.heading')}
            </Typography>
            <Typography variant="subtitle1" gutterBottom>
              {isAuthZENPDP
                ? t(
                    'wizard.configure.pdpSubheading',
                    'Configure the endpoints and runtime settings for this connection.',
                  )
                : t('wizard.configure.subheading')}
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
              showAuthentication={!isAuthZENPDP && activeType !== ConnectionTypes.SMS_GATEWAY}
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
