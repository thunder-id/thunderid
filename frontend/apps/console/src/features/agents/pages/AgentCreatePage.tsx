/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useGetAgentType, useGetAgentTypes} from '@thunderid/configure-agent-types';
import {useGetChildOrganizationUnits} from '@thunderid/configure-organization-units';
import {ConfigureOrganizationUnit} from '@thunderid/configure-users';
import {useLogger} from '@thunderid/logger/react';
import {useThunderID} from '@thunderid/react';
import {Alert, Box, Button, IconButton, LinearProgress, Stack, Typography} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useCreateAgent from '../api/useCreateAgent';
import ConfigureAgentDetails from '../components/create-agent/ConfigureAgentDetails';
import ConfigureName from '../components/create-agent/ConfigureName';
import ConfigureOwner from '../components/create-agent/ConfigureOwner';
import ShowClientSecret from '../components/create-agent/ShowClientSecret';
import useAgentCreate from '../contexts/AgentCreate/useAgentCreate';
import {DEFAULT_AGENT_TYPE_NAME, type Agent, type AgentInboundAuthConfig} from '../models/agent';
import {AgentCreateFlowStep} from '../models/agent-create-flow';
import AppBreadcrumbs from '@/components/AppBreadcrumbs';

export default function AgentCreatePage(): JSX.Element {
  const {t} = useTranslation();
  const navigate = useNavigate();
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

  const agentTypes = useMemo(() => agentTypesData?.types ?? [], [agentTypesData]);
  const [createdAgent, setCreatedAgent] = useState<Agent | null>(null);

  // Agent types are restricted to a single bootstrap-provisioned `default` schema. Auto-pick it
  // once the list loads so the wizard never shows a type-selection step.
  useEffect(() => {
    if (selectedSchema || agentTypes.length === 0) return;
    const defaultType = agentTypes.find((s) => s.name === DEFAULT_AGENT_TYPE_NAME);
    if (defaultType) {
      setSelectedSchema({id: defaultType.id, name: defaultType.name, ouId: defaultType.ouId});
    }
  }, [agentTypes, selectedSchema, setSelectedSchema]);

  const [stepReady, setStepReady] = useState<Record<AgentCreateFlowStep, boolean>>({
    NAME: false,
    ORGANIZATION_UNIT: false,
    PROFILE: true,
    OWNER: true,
  });

  const activeSteps = useMemo((): AgentCreateFlowStep[] => {
    const base: AgentCreateFlowStep[] = [AgentCreateFlowStep.NAME];
    if (hasChildOUs) base.push(AgentCreateFlowStep.ORGANIZATION_UNIT);
    if (schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0) {
      base.push(AgentCreateFlowStep.PROFILE);
    }
    base.push(AgentCreateFlowStep.OWNER);
    return base;
  }, [hasChildOUs, schemaDetails]);

  const steps: Record<AgentCreateFlowStep, {label: string}> = useMemo(
    () => ({
      NAME: {label: t('agents:createWizard.steps.name', 'Name')},
      ORGANIZATION_UNIT: {label: t('agents:createWizard.steps.organizationUnit', 'Organization unit')},
      PROFILE: {label: t('agents:createWizard.steps.profile', 'Profile')},
      OWNER: {label: t('agents:createWizard.steps.owner', 'Owner')},
    }),
    [t],
  );

  const isLastStep = currentStep === activeSteps[activeSteps.length - 1];

  const handleClose = (): void => {
    void navigate('/agents');
  };

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
          token: {
            accessToken: {validityPeriod: 3600, userAttributes: []},
            // idToken is required by the shared OAuth2Token type; default agent grants don't issue
            // ID tokens, but the field must be present to satisfy the type.
            idToken: {validityPeriod: 3600, userAttributes: []},
          },
        },
      },
    ];

    const agentData = {
      ouId,
      type: selectedSchema.name,
      name: agentName,
      ...(selectedOwnerId && {owner: selectedOwnerId}),
      ...(Object.keys(filteredAttributes).length > 0 && {attributes: filteredAttributes}),
      inboundAuthConfig,
    };

    createAgent.mutate(agentData, {
      onSuccess: (created: Agent): void => {
        // The backend always returns a fresh client secret in the create response when an OAuth
        // profile is provisioned. Show the completion screen so the operator can copy it once.
        setCreatedAgent(created);
      },
      onError: (err: Error) => {
        setError(
          err.message ?? t('agents:createWizard.errors.createFailed', 'Failed to create agent. Please try again.'),
        );
      },
    });
  };

  const handleCompleteContinue = (): void => {
    if (!createdAgent) return;
    (async () => {
      await navigate(`/agents/${createdAgent.id}`);
    })().catch((_error: unknown) => {
      logger.error('Failed to navigate to agent details', {error: _error, agentId: createdAgent.id});
    });
  };

  const handleNextStep = (): void => {
    if (isLastStep) {
      handleCreateAgent();
      return;
    }

    switch (currentStep) {
      case AgentCreateFlowStep.NAME: {
        if (selectedSchema?.ouId && isChildOuLoading) return;
        if (isChildOuForbidden) {
          if (tokenOuId) setSelectedOuId(tokenOuId);
        } else if (!hasChildOUs) {
          setSelectedOuId(selectedSchema?.ouId ?? null);
        }
        const hasSchemaFields = schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0;
        if (hasChildOUs) {
          setCurrentStep(AgentCreateFlowStep.ORGANIZATION_UNIT);
        } else {
          setCurrentStep(hasSchemaFields ? AgentCreateFlowStep.PROFILE : AgentCreateFlowStep.OWNER);
        }
        break;
      }
      case AgentCreateFlowStep.ORGANIZATION_UNIT: {
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
      case AgentCreateFlowStep.ORGANIZATION_UNIT:
        setCurrentStep(AgentCreateFlowStep.NAME);
        break;
      case AgentCreateFlowStep.PROFILE:
        if (hasChildOUs) {
          setCurrentStep(AgentCreateFlowStep.ORGANIZATION_UNIT);
        } else {
          setCurrentStep(AgentCreateFlowStep.NAME);
        }
        break;
      case AgentCreateFlowStep.OWNER: {
        const hasSchemaFields = schemaDetails && Object.keys(schemaDetails.schema ?? {}).length > 0;
        if (hasSchemaFields) {
          setCurrentStep(AgentCreateFlowStep.PROFILE);
        } else if (hasChildOUs) {
          setCurrentStep(AgentCreateFlowStep.ORGANIZATION_UNIT);
        } else {
          setCurrentStep(AgentCreateFlowStep.NAME);
        }
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

  const showCompleteScreen = Boolean(
    createdAgent?.inboundAuthConfig?.find((c) => c.type === 'oauth2')?.config?.clientSecret,
  );

  const renderStepContent = (): JSX.Element | null => {
    if (showCompleteScreen && createdAgent) {
      const oauth2Config = createdAgent.inboundAuthConfig?.find((c) => c.type === 'oauth2')?.config;
      const clientSecret = oauth2Config?.clientSecret;
      if (clientSecret) {
        return (
          <ShowClientSecret
            agentName={createdAgent.name}
            clientId={oauth2Config?.clientId}
            clientSecret={clientSecret}
            onContinue={handleCompleteContinue}
          />
        );
      }
    }

    switch (currentStep) {
      case AgentCreateFlowStep.NAME:
        return (
          <ConfigureName
            agentName={agentName}
            onAgentNameChange={setAgentName}
            onReadyChange={(isReady) => handleStepReadyChange(AgentCreateFlowStep.NAME, isReady)}
          />
        );

      case AgentCreateFlowStep.ORGANIZATION_UNIT:
        if (!selectedSchema?.ouId) return null;
        return (
          <ConfigureOrganizationUnit
            key={selectedSchema.ouId}
            rootOuId={selectedSchema.ouId}
            selectedOuId={selectedOuId ?? ''}
            onOuIdChange={setSelectedOuId}
            onReadyChange={(isReady) => handleStepReadyChange(AgentCreateFlowStep.ORGANIZATION_UNIT, isReady)}
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
            onFormValuesChange={setFormValues}
            onReadyChange={(isReady: boolean) => handleStepReadyChange(AgentCreateFlowStep.PROFILE, isReady)}
          />
        );
      }

      case AgentCreateFlowStep.OWNER:
        return (
          <ConfigureOwner
            selectedOwnerId={selectedOwnerId}
            onOwnerIdChange={setSelectedOwnerId}
            onReadyChange={(isReady) => handleStepReadyChange(AgentCreateFlowStep.OWNER, isReady)}
          />
        );

      default:
        return null;
    }
  };

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'column'}}>
        <Box sx={{p: 4, display: 'flex', justifyContent: 'space-between', alignItems: 'center'}}>
          <Stack direction="row" alignItems="center" spacing={2}>
            <IconButton
              onClick={handleClose}
              sx={{
                bgcolor: 'background.paper',
                '&:hover': {bgcolor: 'action.hover'},
                boxShadow: 1,
              }}
            >
              <X size={24} />
            </IconButton>
            <AppBreadcrumbs
              items={getBreadcrumbSteps().map((step, index, array) => ({
                key: step,
                label: steps[step].label,
                onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
              }))}
            />
          </Stack>
        </Box>

        <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
          <Box
            sx={{
              flex: 1,
              display: 'flex',
              flexDirection: 'column',
              pt: showCompleteScreen ? 2 : 8,
              pb: 8,
              px: 20,
              mx: 'auto',
              alignItems: showCompleteScreen ? 'center' : 'flex-start',
              justifyContent: 'flex-start',
            }}
          >
            <Box sx={{width: '100%', maxWidth: 800, display: 'flex', flexDirection: 'column'}}>
              {error && (
                <Alert severity="error" sx={{my: 3}} onClose={() => setError(null)}>
                  {error}
                </Alert>
              )}

              {renderStepContent()}

              {!showCompleteScreen && (
                <Stack direction="row" justifyContent="flex-end" alignItems="center" spacing={2} sx={{mt: 4}}>
                  {currentStep !== AgentCreateFlowStep.NAME && (
                    <Button variant="text" onClick={handlePrevStep} disabled={createAgent.isPending}>
                      {t('common:actions.back')}
                    </Button>
                  )}
                  <Button
                    variant="contained"
                    disabled={
                      !stepReady[currentStep] ||
                      createAgent.isPending ||
                      (currentStep === AgentCreateFlowStep.NAME && Boolean(selectedSchema?.ouId) && isChildOuLoading)
                    }
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
              )}
            </Box>
          </Box>
        </Box>
      </Box>
    </Box>
  );
}
