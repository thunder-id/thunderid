// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import {
  OrganizationUnitPickerScreen,
  useGetChildOrganizationUnits,
  useGetOrganizationUnit,
} from '@thunderid/configure-organization-units';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import {getErrorMessage} from '@thunderid/utils';
import {Alert, Box, Button, CircularProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {Home} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import useCreateAgent from '../api/useCreateAgent';
import ConfigureAgentDetails from '../components/create-agent/ConfigureAgentDetails';
import ConfigureName from '../components/create-agent/ConfigureName';
import ConfigureOwner from '../components/create-agent/ConfigureOwner';
import AgentConstants from '../constants/agent-constants';
import useAgentCreate from '../contexts/AgentCreate/useAgentCreate';
import {DEFAULT_AGENT_TYPE_NAME, type Agent, type AgentInboundAuthConfig} from '../models/agent';
import {AgentCreateFlowStep} from '../models/agent-create-flow';

export default function AgentCreatePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const isWelcomeFlow = pathname.startsWith('/welcome');
  const logger = useLogger('AgentCreatePage');
  const createAgent = useCreateAgent();

  const {
    currentStep,
    setCurrentStep,
    selectedSchema,
    setSelectedSchema,
    selectedOuId,
    setSelectedOuId,
    agentName,
    setAgentName,
    formValues,
    setFormValues,
    selectedOwnerId,
    setSelectedOwnerId,
    error,
    setError,
  } = useAgentCreate();

  const {data: agentTypesData} = useGetAgentTypes();
  const {data: userTypesData} = useGetUserTypes();
  const {data: schemaDetails, isLoading: isSchemaLoading} = useGetAgentType(selectedSchema?.id);
  const {
    data: childOuData,
    isLoading: isChildOuLoading,
    error: childOuError,
  } = useGetChildOrganizationUnits(selectedSchema?.ouId, {limit: 1, offset: 0});
  const user = useThunderID().user as {ouId?: string} | null | undefined;
  const tokenOuId = user?.ouId ?? null;
  const isChildOuForbidden = (childOuError as {response?: {status?: number}} | null)?.response?.status === 403;
  const hasChildOUs = !isChildOuLoading && !childOuError && (childOuData?.totalResults ?? 0) > 0;

  // The organization unit whose name is shown in the Details step's summary chip: the picked one
  // when there was a choice to make, or the schema's own OU otherwise.
  const resolvedOuId = selectedOuId ?? selectedSchema?.ouId;
  const {data: resolvedOrganizationUnit, isLoading: isResolvedOuLoading} = useGetOrganizationUnit(
    resolvedOuId,
    Boolean(resolvedOuId),
  );

  const agentTypes = useMemo(() => agentTypesData?.types ?? [], [agentTypesData]);

  // Agent types are restricted to a single bootstrap-provisioned `default` schema. Auto-pick it
  // once the list loads so the wizard never shows a type-selection step.
  useEffect(() => {
    if (selectedSchema || agentTypes.length === 0) return;
    const defaultType = agentTypes.find((s) => s.name === DEFAULT_AGENT_TYPE_NAME);
    if (defaultType) {
      setSelectedSchema({id: defaultType.id, name: defaultType.name, ouId: defaultType.ouId});
    }
  }, [agentTypes, selectedSchema, setSelectedSchema]);

  // The organization unit is the wizard's first step, scoped to the agent type's own OU subtree,
  // whenever there's an actual choice to make (the schema's OU has children). When there's no
  // choice, the schema's own OU (or, if the lookup is forbidden, the caller's token OU) is
  // resolved automatically and the wizard skips straight to the Name step.
  useEffect(() => {
    if (!selectedSchema?.ouId || isChildOuLoading || currentStep !== AgentCreateFlowStep.ORGANIZATION_UNIT) return;
    if (isChildOuForbidden) {
      if (tokenOuId) setSelectedOuId(tokenOuId);
      setCurrentStep(AgentCreateFlowStep.NAME);
    } else if (!hasChildOUs) {
      setSelectedOuId(selectedSchema.ouId);
      setCurrentStep(AgentCreateFlowStep.NAME);
    }
  }, [
    selectedSchema,
    isChildOuLoading,
    isChildOuForbidden,
    hasChildOUs,
    tokenOuId,
    currentStep,
    setSelectedOuId,
    setCurrentStep,
  ]);

  const [stepReady, setStepReady] = useState<Record<AgentCreateFlowStep, boolean>>({
    ORGANIZATION_UNIT: false,
    NAME: false,
    PROFILE: true,
    OWNER: true,
  });

  const activeSteps = useMemo((): AgentCreateFlowStep[] => {
    const base: AgentCreateFlowStep[] = [];
    if (hasChildOUs) base.push(AgentCreateFlowStep.ORGANIZATION_UNIT);
    base.push(AgentCreateFlowStep.NAME);
    if (schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0) {
      base.push(AgentCreateFlowStep.PROFILE);
    }
    base.push(AgentCreateFlowStep.OWNER);
    return base;
  }, [hasChildOUs, schemaDetails]);

  const steps: Partial<Record<AgentCreateFlowStep, {label: string}>> = useMemo(() => {
    const map: Partial<Record<AgentCreateFlowStep, {label: string}>> = {};
    if (hasChildOUs) {
      map.ORGANIZATION_UNIT = {label: t('agents:createWizard.steps.organizationUnit', 'Organization unit')};
    }
    map.NAME = {label: t('agents:createWizard.steps.name', 'Details')};
    map.PROFILE = {label: t('agents:createWizard.steps.profile', 'Profile')};
    map.OWNER = {label: t('agents:createWizard.steps.owner', 'Owner')};
    return map;
  }, [t, hasChildOUs]);

  const isLastStep = currentStep === activeSteps[activeSteps.length - 1];

  const handleClose = (): void => {
    void navigate(isWelcomeFlow ? RouteConfig.home.list() : RouteConfig.agents.list());
  };

  // Leaving the wizard backwards, as opposed to dismissing it: the welcome flow reached it from
  // the Get Started page, so that's where Back belongs there.
  const handleBackOutOfWizard = (): void => {
    void navigate(isWelcomeFlow ? RouteConfig.welcome.getStarted() : RouteConfig.agents.list());
  };

  const prefixCrumbs = isWelcomeFlow
    ? [
        {key: 'welcome', label: t('common:welcome.header'), onClick: () => void navigate(RouteConfig.welcome.root())},
        {
          key: 'new',
          label: t('common:welcome.createProject.breadcrumb'),
          onClick: () => void navigate(RouteConfig.welcome.createProject()),
        },
        {
          key: 'get-started',
          label: t('common:welcome.getStarted.breadcrumb'),
          onClick: () => void navigate(RouteConfig.welcome.getStarted()),
        },
      ]
    : [
        {
          key: 'agents',
          label: t('agents:listing.title', 'Agents'),
          onClick: () => void navigate(RouteConfig.agents.list()),
        },
      ];

  // Resolves an error through the `agents` catalog. `t` defaults to the `common` namespace, so
  // this forwards explicit `ns:` prefixes unchanged and prefixes bare keys with `agents:`, per
  // getErrorMessage's namespace-resolution contract.
  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `agents:${key}`, options),
    [t],
  );

  // A create failure is stale once the user edits any field, so every field-change path below
  // clears both the validation error and the mutation's own error before applying the change.
  // Only reset the mutation once it has actually failed: resetting while it's still pending would
  // flip isPending back to false and re-enable the submit button before the in-flight request
  // settles, letting the user fire a second concurrent create.
  const clearCreateError = useCallback((): void => {
    setError(null);
    if (createAgent.isError) {
      createAgent.reset();
    }
  }, [setError, createAgent]);

  const handleAgentNameChange = useCallback(
    (newName: string): void => {
      clearCreateError();
      setAgentName(newName);
    },
    [clearCreateError, setAgentName],
  );

  const handleOuIdChange = useCallback(
    (newOuId: string): void => {
      clearCreateError();
      setSelectedOuId(newOuId);
    },
    [clearCreateError, setSelectedOuId],
  );

  const handleFormValuesChange = useCallback(
    (newFormValues: typeof formValues): void => {
      clearCreateError();
      setFormValues(newFormValues);
    },
    [clearCreateError, setFormValues],
  );

  const handleOwnerIdChange = useCallback(
    (newOwnerId: string | null): void => {
      clearCreateError();
      setSelectedOwnerId(newOwnerId);
    },
    [clearCreateError, setSelectedOwnerId],
  );

  const handleStepReadyChange = useCallback((step: AgentCreateFlowStep, isReady: boolean): void => {
    setStepReady((prev) => (prev[step] === isReady ? prev : {...prev, [step]: isReady}));
  }, []);

  const handleCreateAgent = (): void => {
    setError(null);

    const ouId = selectedOuId ?? selectedSchema?.ouId;
    if (!ouId) {
      setError(t('agents:createWizard.errors.ouRequired', 'Organization unit is required'));
      return;
    }
    if (!selectedSchema) {
      setError(t('agents:createWizard.errors.schemaRequired', 'Schema is required'));
      return;
    }

    const filteredAttributes = Object.fromEntries(
      Object.entries(formValues).filter(([, v]) => v !== '' && v !== undefined && v !== null),
    );

    // OAuth is always provisioned for new agents — backend issues a client ID + secret which we
    // surface on the completion screen.
    const inboundAuthConfig: AgentInboundAuthConfig[] = [
      {
        type: 'oauth2',
        config: {
          grantTypes: ['client_credentials'],
          tokenEndpointAuthMethod: 'client_secret_basic',
          responseTypes: [],
          // PKCE requires the authorization_code grant, which new agents don't have yet.
          pkceRequired: false,
          token: {
            accessToken: {userConfig: {validityPeriod: 3600, attributes: []}},
            // idToken is required by the shared OAuth2Token type; default agent grants don't issue
            // ID tokens, but the field must be present to satisfy the type.
            idToken: {validityPeriod: 3600, userAttributes: []},
          },
        },
      },
    ];

    // The user types an agent may act on behalf of. With exactly one deployed there is nothing to
    // choose, and sending it lets the server derive the agent's token attributes from its schema at
    // creation, rather than leaving them empty until delegated mode is turned on later.
    const userTypes = userTypesData?.types ?? [];
    const allowedUserTypes = userTypes.length === 1 ? [userTypes[0].name] : undefined;

    const agentData = {
      ouId,
      type: selectedSchema.name,
      name: agentName,
      logoUrl: AgentConstants.DEFAULT_AVATAR,
      ...(allowedUserTypes && {allowedUserTypes}),
      ...(selectedOwnerId && {owner: selectedOwnerId}),
      ...(Object.keys(filteredAttributes).length > 0 && {attributes: filteredAttributes}),
      inboundAuthConfig,
    };

    createAgent.mutate(agentData, {
      onSuccess: (created: Agent): void => {
        // The backend always returns a fresh client secret in the create response when an OAuth
        // profile is provisioned. Surface it as a popup on the agent's detail page.
        const oauth2Config = created.inboundAuthConfig?.find((c) => c.type === 'oauth2')?.config;
        const clientSecret = oauth2Config?.clientSecret;
        (async () => {
          if (clientSecret) {
            await navigate(RouteConfig.agents.detail(created.id), {
              state: {justCreatedSecret: {agentName: created.name, clientId: oauth2Config?.clientId, clientSecret}},
            });
          } else {
            await navigate(RouteConfig.agents.detail(created.id));
          }
        })().catch((_error: unknown) => {
          logger.error('Failed to navigate to agent details', {error: _error, agentId: created.id});
        });
      },
      onError: (err: Error) => {
        setError(
          getErrorMessage(
            err,
            tForErrors,
            'createWizard.errors.createFailed',
            'Failed to create agent. Please try again.',
          ),
        );
      },
    });
  };

  const handleNextStep = (): void => {
    if (isLastStep) {
      handleCreateAgent();
      return;
    }

    switch (currentStep) {
      case AgentCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(AgentCreateFlowStep.NAME);
        break;
      case AgentCreateFlowStep.NAME: {
        const hasSchemaFields = schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0;
        setCurrentStep(hasSchemaFields ? AgentCreateFlowStep.PROFILE : AgentCreateFlowStep.OWNER);
        break;
      }
      case AgentCreateFlowStep.PROFILE:
        setCurrentStep(AgentCreateFlowStep.OWNER);
        break;
      default:
        break;
    }
  };

  const handlePrevStep = (): void => {
    switch (currentStep) {
      case AgentCreateFlowStep.NAME:
        if (hasChildOUs) setCurrentStep(AgentCreateFlowStep.ORGANIZATION_UNIT);
        break;
      case AgentCreateFlowStep.PROFILE:
        setCurrentStep(AgentCreateFlowStep.NAME);
        break;
      case AgentCreateFlowStep.OWNER: {
        const hasSchemaFields = schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0;
        setCurrentStep(hasSchemaFields ? AgentCreateFlowStep.PROFILE : AgentCreateFlowStep.NAME);
        break;
      }
      default:
        break;
    }
  };

  const getStepProgress = (): number => {
    const idx = activeSteps.indexOf(currentStep);
    return ((idx + 1) / (activeSteps.length + 1)) * 100;
  };

  const getBreadcrumbSteps = (): AgentCreateFlowStep[] => {
    const idx = activeSteps.indexOf(currentStep);
    return activeSteps.slice(0, idx + 1);
  };

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case AgentCreateFlowStep.NAME:
        return (
          <ConfigureName
            agentName={agentName}
            onAgentNameChange={handleAgentNameChange}
            onReadyChange={(isReady) => handleStepReadyChange(AgentCreateFlowStep.NAME, isReady)}
            hasChildOUs={hasChildOUs}
            organizationUnitName={resolvedOrganizationUnit?.name}
            organizationUnitLogoUrl={resolvedOrganizationUnit?.logoUrl}
            isOrganizationUnitLoading={isResolvedOuLoading}
            onChangeOu={() => setCurrentStep(AgentCreateFlowStep.ORGANIZATION_UNIT)}
          />
        );

      case AgentCreateFlowStep.PROFILE: {
        if (isSchemaLoading) {
          return (
            <Box sx={{textAlign: 'center', py: 4}}>
              <Typography variant="body2" color="text.secondary">
                {t('common:status.loading')}
              </Typography>
            </Box>
          );
        }
        if (!schemaDetails) return null;

        return (
          <ConfigureAgentDetails
            key={selectedSchema?.id}
            schema={schemaDetails}
            defaultValues={formValues}
            onFormValuesChange={handleFormValuesChange}
            onReadyChange={(isReady: boolean) => handleStepReadyChange(AgentCreateFlowStep.PROFILE, isReady)}
          />
        );
      }

      case AgentCreateFlowStep.OWNER:
        return (
          <ConfigureOwner
            selectedOwnerId={selectedOwnerId}
            onOwnerIdChange={handleOwnerIdChange}
            onReadyChange={(isReady) => handleStepReadyChange(AgentCreateFlowStep.OWNER, isReady)}
          />
        );

      default:
        return null;
    }
  };

  if (currentStep === AgentCreateFlowStep.ORGANIZATION_UNIT) {
    if (!selectedSchema || isChildOuLoading || !hasChildOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <CircularProgress />
        </Box>
      );
    }

    return (
      <OrganizationUnitPickerScreen
        rootOuId={selectedSchema.ouId}
        icon={<Home size={26} />}
        title={t('agents:createWizard.organizationUnit.title', 'Where should this agent belong?')}
        subtitle={t(
          'agents:createWizard.organizationUnit.subtitle',
          "Choose the organization unit that will own this agent. You can't change this once created.",
        )}
        value={selectedOuId ?? ''}
        onChange={handleOuIdChange}
        onBack={handleBackOutOfWizard}
        onContinue={handleNextStep}
        backLabel={t('common:actions.back', 'Back')}
        continueLabel={t('common:actions.continue', 'Continue')}
      />
    );
  }

  return (
    <FullScreenCreationWizardLayout
      onClose={handleClose}
      progress={getStepProgress()}
      breadcrumbItems={[
        ...prefixCrumbs,
        ...getBreadcrumbSteps().map((step, index, array) => ({
          key: step,
          label: steps[step]?.label ?? step,
          onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
        })),
      ]}
      footer={
        <Stack
          direction="row"
          justifyContent={activeSteps.indexOf(currentStep) > 0 ? 'space-between' : 'flex-end'}
          alignItems="center"
          spacing={2}
        >
          {activeSteps.indexOf(currentStep) > 0 && (
            <Button variant="outlined" onClick={handlePrevStep} sx={{minWidth: 100}} disabled={createAgent.isPending}>
              {t('common:actions.back')}
            </Button>
          )}
          <Button
            variant="contained"
            disabled={!stepReady[currentStep] || createAgent.isPending}
            sx={{minWidth: 140}}
            onClick={handleNextStep}
          >
            {(() => {
              if (!isLastStep) return t('common:actions.continue');
              if (createAgent.isPending) return t('common:status.saving');
              return t('agents:createWizard.createAgent', 'Create agent');
            })()}
          </Button>
        </Stack>
      }
    >
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={clearCreateError}>
          {error}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
