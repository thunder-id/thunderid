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

import {useHasMultipleOUs} from '@thunderid/configure-organization-units';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {useLogger} from '@thunderid/logger/react';
import {Box, Stack, Button, IconButton, LinearProgress, Alert, CircularProgress, AppBreadcrumbs} from '@wso2/oxygen-ui';
import {X} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useCallback, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import useCreateFlow from '../../flows/api/useCreateFlow';
import useGetFlowById from '../../flows/api/useGetFlowById';
import type {BasicFlowDefinition} from '../../flows/models/responses';
import generateFlowGraph from '../../flows/utils/generateFlowGraph';
import getFlowEntryComponents from '../../flows/utils/getFlowEntryComponents';
import useIdentityProviders from '../../integrations/api/useIdentityProviders';
import {AuthenticatorTypes} from '../../integrations/models/authenticators';
import {IdentityProviderTypes} from '../../integrations/models/identity-provider';
import useCreateApplication from '../api/useCreateApplication';
import ConfigureSignInOptions from '../components/create-application/configure-signin-options/ConfigureSignInOptions';
import ConfigureDesign from '../components/create-application/ConfigureDesign';
import ConfigureDetails from '../components/create-application/ConfigureDetails';
import ConfigureExperience from '../components/create-application/ConfigureExperience';
import ConfigureName from '../components/create-application/ConfigureName';
import ConfigureOrganizationUnit from '../components/create-application/ConfigureOrganizationUnit';
import ConfigureStack from '../components/create-application/ConfigureStack';
import ShowClientSecret from '../components/create-application/ShowClientSecret';
import TemplateConstants from '../constants/template-constants';
import useApplicationCreate from '../contexts/ApplicationCreate/useApplicationCreate';
import type {Application} from '../models/application';
import {
  ApplicationCreateFlowConfiguration,
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowStep,
} from '../models/application-create-flow';
import {PlatformApplicationTemplate} from '../models/application-templates';
import type {OAuth2Config} from '../models/oauth';
import type {CreateApplicationRequest} from '../models/requests';
import getConfigurationTypeFromTemplate from '../utils/getConfigurationTypeFromTemplate';
import resolveCreationFlow from '../utils/resolveCreationFlow';
import GatePreview from '@/components/GatePreview/GatePreview';
import buildPreviewMock from '@/components/GatePreview/mocks/buildPreviewMock';

export default function ApplicationCreatePage(): JSX.Element {
  const {t} = useTranslation();

  const {
    currentStep,
    setCurrentStep,
    appName,
    setAppName,
    ouId,
    setOuId,
    themeId,
    setThemeId,
    selectedTheme,
    setSelectedTheme,
    appLogo,
    setAppLogo,
    integrations,
    toggleIntegration,
    selectedAuthFlow,
    setSelectedAuthFlow,
    signInApproach,
    setSignInApproach,
    selectedTechnology,
    selectedPlatform,
    hostingUrl,
    setHostingUrl,
    callbackUrlFromConfig,
    setCallbackUrlFromConfig,
    relyingPartyId,
    relyingPartyName,
    selectedTemplateConfig,
    error,
    setError,
  } = useApplicationCreate();

  const steps: Record<ApplicationCreateFlowStep, {label: string; order: number}> = useMemo(
    () => ({
      STACK: {label: t('applications:onboarding.steps.stack'), order: 1},
      NAME: {label: t('applications:onboarding.steps.name'), order: 2},
      ORGANIZATION_UNIT: {label: t('applications:onboarding.steps.organizationUnit'), order: 3},
      DESIGN: {label: t('applications:onboarding.steps.design'), order: 4},
      OPTIONS: {label: t('applications:onboarding.steps.options'), order: 5},
      EXPERIENCE: {label: t('applications:onboarding.steps.experience'), order: 6},
      CONFIGURE: {label: t('applications:onboarding.steps.configure'), order: 7},
      COMPLETE: {label: t('applications:onboarding.steps.complete'), order: 8},
    }),
    [t],
  );
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const isWelcomeFlow = pathname.startsWith('/welcome');
  const logger = useLogger('ApplicationCreatePage');
  const createApplication = useCreateApplication();
  const {data: userTypesData} = useGetUserTypes();
  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const [selectedUserTypes, setSelectedUserTypes] = useState<string[]>([]);
  const [createdApplication, setCreatedApplication] = useState<Application | null>(null);

  const createFlow = useCreateFlow();
  const {data: idpData} = useIdentityProviders();

  const hasEnabledIntegrations = useMemo((): boolean => Object.values(integrations).some(Boolean), [integrations]);

  const previewFlowId: string | undefined =
    !hasEnabledIntegrations && selectedAuthFlow ? selectedAuthFlow.id : undefined;
  const {data: previewFlow, isLoading: isPreviewFlowLoading} = useGetFlowById(previewFlowId, Boolean(previewFlowId));

  const previewMock = useMemo(() => {
    if (hasEnabledIntegrations || !selectedAuthFlow) {
      return buildPreviewMock(integrations, idpData ?? [], {
        application: {
          logoUrl: appLogo!,
        },
      });
    }

    return getFlowEntryComponents(previewFlow) ?? [];
  }, [hasEnabledIntegrations, selectedAuthFlow, integrations, idpData, appLogo, previewFlow]);

  const [stepReady, setStepReady] = useState<Record<ApplicationCreateFlowStep, boolean>>({
    STACK: true,
    NAME: false,
    ORGANIZATION_UNIT: false,
    DESIGN: true,
    OPTIONS: true,
    EXPERIENCE: true,
    CONFIGURE: true,
    COMPLETE: true,
  });

  const [oauthConfig, setOAuthConfig] = useState<OAuth2Config | null>(null);
  const [walletClientId, setWalletClientId] = useState<string>('');

  const effectiveOauthConfig = useMemo(() => {
    if (!oauthConfig) return oauthConfig;
    let config: OAuth2Config = callbackUrlFromConfig
      ? {...oauthConfig, redirectUris: [callbackUrlFromConfig]}
      : oauthConfig;
    if (walletClientId.trim()) {
      config = {...config, clientId: walletClientId.trim()};
    }
    return config;
  }, [oauthConfig, callbackUrlFromConfig, walletClientId]);

  const creationFlow = useMemo(() => resolveCreationFlow(selectedTemplateConfig), [selectedTemplateConfig]);

  // Browser-based SPAs are public clients that must use the redirect-based flow, so the
  // embedded (native) sign-in approach is not offered for them. Native mobile apps and digital
  // wallets are also public clients but legitimately use app-native flows, so they are excluded
  // from this rule.
  const isBrowserSpaTemplate = useMemo((): boolean => {
    if (
      selectedPlatform === PlatformApplicationTemplate.MOBILE ||
      selectedPlatform === PlatformApplicationTemplate.WALLET
    ) {
      return false;
    }
    const oauthConfig = selectedTemplateConfig?.defaults?.inboundAuthConfig?.find(
      (config) => config.type === 'oauth2',
    )?.config;
    return oauthConfig?.publicClient === true;
  }, [selectedTemplateConfig, selectedPlatform]);

  const needsConfigure = useMemo((): boolean => {
    const isPasskeyEnabled = !selectedAuthFlow && (integrations[AuthenticatorTypes.PASSKEY] ?? false);
    if (signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED) {
      return isPasskeyEnabled;
    }
    return (
      getConfigurationTypeFromTemplate(selectedTemplateConfig) !== ApplicationCreateFlowConfiguration.NONE ||
      isPasskeyEnabled
    );
  }, [selectedTemplateConfig, integrations, signInApproach, selectedAuthFlow]);

  const visibleSteps = useMemo((): ApplicationCreateFlowStep[] => {
    return creationFlow.steps.filter((step) => {
      if (step === ApplicationCreateFlowStep.ORGANIZATION_UNIT) return hasMultipleOUs;
      if (step === ApplicationCreateFlowStep.CONFIGURE) return needsConfigure;
      // COMPLETE step is dynamic — only shown when an app with a client secret is created.
      // It's filtered out of visibleSteps here and is only set explicitly via setCurrentStep
      // when the creation succeeds with a client secret.
      if (step === ApplicationCreateFlowStep.COMPLETE) return false;
      return true;
    });
  }, [creationFlow, hasMultipleOUs, needsConfigure]);

  const handleClose = (): void => {
    void navigate(isWelcomeFlow ? '/home' : '/applications');
  };

  const handleLogoSelect = (logoUrl: string): void => {
    setAppLogo(logoUrl);
  };

  const handleIntegrationToggle = (integrationId: string): void => {
    toggleIntegration(integrationId);
  };

  const handleCreateApplication = (skipOAuthConfig = false, overrideFlowId?: string): void => {
    setError(null);

    const includesOptions = creationFlow.steps.includes(ApplicationCreateFlowStep.OPTIONS);
    const includesDesign = creationFlow.steps.includes(ApplicationCreateFlowStep.DESIGN);
    const includesExperience = creationFlow.steps.includes(ApplicationCreateFlowStep.EXPERIENCE);

    const authFlowId: string | undefined = overrideFlowId ?? selectedAuthFlow?.id;

    // authFlowId is only required when the flow has user-facing sign-in steps
    if (includesOptions && !authFlowId) {
      setError(t('onboarding.configure.SignInOptions.noFlowFound'));
      return;
    }

    const userTypes = userTypesData?.types ?? [];
    const allowedUserTypes = (() => {
      if (userTypes.length === 1) return [userTypes[0].name];
      if (userTypes.length > 1) return selectedUserTypes.length > 0 ? selectedUserTypes : undefined;
      return undefined;
    })();

    const effectiveOuId = hasMultipleOUs ? ouId : ouList[0]?.id;

    const templateId = selectedTemplateConfig?.id;
    const finalTemplateId =
      templateId && includesExperience && signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED
        ? `${templateId}${TemplateConstants.EMBEDDED_SUFFIX}`
        : templateId;

    const applicationData: CreateApplicationRequest = {
      name: appName,
      ...(hostingUrl && {url: hostingUrl}),
      ...(authFlowId && {authFlowId}),
      ...(effectiveOuId && {ouId: effectiveOuId}),
      ...(finalTemplateId && {template: finalTemplateId}),
      ...(includesDesign && {
        logoUrl: appLogo ?? undefined,
        ...(themeId && {themeId}),
      }),
      ...(includesOptions && {
        userAttributes: ['given_name', 'family_name', 'email', 'groups'],
        isRegistrationFlowEnabled: true,
      }),
      ...(includesExperience && allowedUserTypes && {allowedUserTypes}),
      ...(!skipOAuthConfig && {
        inboundAuthConfig: [{type: 'oauth2', config: effectiveOauthConfig}],
      }),
    };

    createApplication.mutate(applicationData, {
      onSuccess: (createdApp: Application): void => {
        const hasClientSecret = createdApp.inboundAuthConfig?.some(
          (config) => config.type === 'oauth2' && config.config?.clientSecret,
        );

        if (hasClientSecret) {
          setCreatedApplication(createdApp);
          setCurrentStep(ApplicationCreateFlowStep.COMPLETE);
        } else {
          (async () => {
            await navigate(`/applications/${createdApp.id}`);
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to application details', {error: _error, applicationId: createdApp.id});
          });
        }
      },
      onError: (err: Error) => {
        setError(err.message ?? 'Failed to create application. Please try again.');
      },
    });
  };

  const ensureFlowAndCreateApplication = (skipOAuthConfig = false): void => {
    // If we already have a selected flow, proceed to create application
    if (selectedAuthFlow) {
      handleCreateApplication(skipOAuthConfig);
      return;
    }

    // Check if we need to generate a flow
    const hasEnabledIntegrations = Object.values(integrations).some((v) => v);

    if (hasEnabledIntegrations) {
      const availableIntegrations = idpData ?? [];
      const googleProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GOOGLE);
      const githubProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GITHUB);

      const generatedFlowRequest = generateFlowGraph({
        hasCredentialsAuth: integrations[AuthenticatorTypes.CREDENTIALS_AUTH] ?? false,
        hasPasskey: integrations[AuthenticatorTypes.PASSKEY] ?? false,
        googleIdpId: integrations[googleProvider?.id ?? ''] ? googleProvider?.id : undefined,
        githubIdpId: integrations[githubProvider?.id ?? ''] ? githubProvider?.id : undefined,
        hasSmsOtp: integrations['sms-otp'] ?? false,
        relyingPartyId: relyingPartyId || window.location.hostname,
        relyingPartyName: relyingPartyName || appName,
      });

      createFlow.mutate(generatedFlowRequest, {
        onSuccess: (savedFlow) => {
          // We cast because BasicFlowDefinition is a subset of FlowDefinitionResponse
          setSelectedAuthFlow(savedFlow as unknown as BasicFlowDefinition);

          // Proceed to create application with the newly generated flow
          handleCreateApplication(skipOAuthConfig, savedFlow.id);
        },
        onError: (err) => {
          setError(err.message ?? 'Failed to generate authentication flow.');
        },
      });
    } else {
      // If no integrations selected, try to create application (will fail validation if flow required)
      handleCreateApplication(skipOAuthConfig);
    }
  };

  const handleNextStep = (): void => {
    // COMPLETE is a terminal step after creation — handled separately
    if (currentStep === ApplicationCreateFlowStep.COMPLETE) {
      if (createdApplication) {
        (async () => {
          await navigate(`/applications/${createdApplication.id}`);
        })().catch((_error: unknown) => {
          logger.error('Failed to navigate to application details', {
            error: _error,
            applicationId: createdApplication.id,
          });
        });
      }
      return;
    }

    // NAME has a special wait condition for OU loading
    if (currentStep === ApplicationCreateFlowStep.NAME && isOuLoading) return;

    const idx = visibleSteps.indexOf(currentStep);
    const next = visibleSteps[idx + 1];

    if (next === undefined) {
      // No more visible steps → create the application
      const includesOptions = creationFlow.steps.includes(ApplicationCreateFlowStep.OPTIONS);
      const includesExperience = creationFlow.steps.includes(ApplicationCreateFlowStep.EXPERIENCE);
      const skipOAuth = includesExperience && signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED;

      if (includesOptions) {
        // user-facing flow → may need to generate the auth flow from integrations
        ensureFlowAndCreateApplication(skipOAuth);
      } else {
        // m2m flow → no integrations; server assigns default flow
        handleCreateApplication(false);
      }
    } else {
      setCurrentStep(next);
    }
  };

  const handlePrevStep = (): void => {
    const idx = visibleSteps.indexOf(currentStep);
    const prev = visibleSteps[idx - 1];
    if (prev) setCurrentStep(prev);
  };

  const handleStepReadyChange = useCallback((step: ApplicationCreateFlowStep, isReady: boolean): void => {
    setStepReady((prev) => ({
      ...prev,
      [step]: isReady,
    }));
  }, []);

  const handleNameStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.NAME, isReady);
    },
    [handleStepReadyChange],
  );

  const handleOuStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.ORGANIZATION_UNIT, isReady);
    },
    [handleStepReadyChange],
  );

  const handleDesignStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.DESIGN, isReady);
    },
    [handleStepReadyChange],
  );

  const handleOptionsStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.OPTIONS, isReady);
    },
    [handleStepReadyChange],
  );

  const handleApproachStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.EXPERIENCE, isReady);
    },
    [handleStepReadyChange],
  );

  const handleTechnologyStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.STACK, isReady);
    },
    [handleStepReadyChange],
  );

  const handleConfigureStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.CONFIGURE, isReady);
    },
    [handleStepReadyChange],
  );

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case ApplicationCreateFlowStep.NAME:
        return (
          <ConfigureName appName={appName} onAppNameChange={setAppName} onReadyChange={handleNameStepReadyChange} />
        );

      case ApplicationCreateFlowStep.ORGANIZATION_UNIT:
        return (
          <ConfigureOrganizationUnit
            selectedOuId={ouId}
            onOuIdChange={setOuId}
            onReadyChange={handleOuStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.DESIGN:
        return (
          <ConfigureDesign
            appLogo={appLogo}
            themeId={themeId}
            selectedTheme={selectedTheme}
            onLogoSelect={handleLogoSelect}
            onThemeSelect={(id, config) => {
              setThemeId(id);
              setSelectedTheme(config);
            }}
            onReadyChange={handleDesignStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.OPTIONS:
        return (
          <ConfigureSignInOptions
            integrations={integrations}
            onIntegrationToggle={handleIntegrationToggle}
            onReadyChange={handleOptionsStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.EXPERIENCE:
        return (
          <ConfigureExperience
            selectedApproach={signInApproach}
            onApproachChange={setSignInApproach}
            allowEmbeddedApproach={!isBrowserSpaTemplate}
            onReadyChange={handleApproachStepReadyChange}
            userTypes={userTypesData?.types ?? []}
            selectedUserTypes={selectedUserTypes}
            onUserTypesChange={setSelectedUserTypes}
          />
        );

      case ApplicationCreateFlowStep.STACK:
        return (
          <ConfigureStack
            oauthConfig={oauthConfig}
            onOAuthConfigChange={setOAuthConfig}
            onReadyChange={handleTechnologyStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.CONFIGURE:
        return (
          <ConfigureDetails
            technology={selectedTechnology}
            platform={selectedPlatform}
            onHostingUrlChange={setHostingUrl}
            onCallbackUrlChange={setCallbackUrlFromConfig}
            onClientIdChange={setWalletClientId}
            onReadyChange={handleConfigureStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.COMPLETE: {
        if (!createdApplication) {
          return null;
        }

        const oauth2Config = createdApplication.inboundAuthConfig?.find((config) => config.type === 'oauth2');
        const clientId = oauth2Config?.config?.clientId;
        const clientSecret = oauth2Config?.config?.clientSecret;

        if (!clientSecret) {
          return null;
        }

        return (
          <ShowClientSecret
            appName={appName}
            clientId={clientId}
            clientSecret={clientSecret}
            onContinue={handleNextStep}
          />
        );
      }

      default:
        return null;
    }
  };

  const getStepProgress = (): number => {
    const stepNames = Object.keys(steps) as ApplicationCreateFlowStep[];
    return ((stepNames.indexOf(currentStep) + 1) / stepNames.length) * 100;
  };

  const getBreadcrumbSteps = (): ApplicationCreateFlowStep[] => {
    const idx = visibleSteps.indexOf(currentStep);
    if (idx < 0) return visibleSteps;
    return visibleSteps.slice(0, idx + 1);
  };

  const prefixCrumbs = isWelcomeFlow
    ? [
        {key: 'welcome', label: t('common:welcome.header'), onClick: () => void navigate('/welcome')},
        {
          key: 'new',
          label: t('common:welcome.createProject.breadcrumb'),
          onClick: () => void navigate('/welcome/create-project'),
        },
        {
          key: 'get-started',
          label: t('common:welcome.getStarted.breadcrumb'),
          onClick: () => void navigate('/welcome/get-started'),
        },
      ]
    : [{key: 'applications', label: t('navigation:pages.applications'), onClick: () => void navigate('/applications')}];

  return (
    <Box sx={{minHeight: '100vh', display: 'flex', flexDirection: 'column'}}>
      {/* Progress bar at the very top */}
      <LinearProgress variant="determinate" value={getStepProgress()} sx={{height: 6}} />

      <Box sx={{flex: 1, display: 'flex', flexDirection: 'row'}}>
        <Box
          sx={{
            flex:
              currentStep === ApplicationCreateFlowStep.STACK ||
              currentStep === ApplicationCreateFlowStep.NAME ||
              currentStep === ApplicationCreateFlowStep.ORGANIZATION_UNIT ||
              currentStep === ApplicationCreateFlowStep.COMPLETE
                ? 1
                : '0 0 50%',
            display: 'flex',
            flexDirection: 'column',
          }}
        >
          {/* Header with close button and breadcrumb */}
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
                items={[
                  ...prefixCrumbs,
                  ...getBreadcrumbSteps().map((step, index, array) => ({
                    key: step,
                    label: steps[step].label,
                    onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
                  })),
                ]}
              />
            </Stack>
          </Box>

          {/* Main content */}
          <Box sx={{flex: 1, display: 'flex', minHeight: 0}}>
            {/* Left side - Form content */}
            <Box
              sx={{
                flex: 1,
                display: 'flex',
                flexDirection: 'column',
                py: 8,
                px: 20,
                mx:
                  currentStep === ApplicationCreateFlowStep.STACK ||
                  currentStep === ApplicationCreateFlowStep.NAME ||
                  currentStep === ApplicationCreateFlowStep.ORGANIZATION_UNIT
                    ? 'auto'
                    : 0,
                alignItems: currentStep === ApplicationCreateFlowStep.COMPLETE ? 'center' : 'flex-start',
              }}
            >
              <Box
                sx={{
                  width: '100%',
                  maxWidth: {xs: '100%', md: currentStep === ApplicationCreateFlowStep.STACK ? '70%' : 800},
                  display: 'flex',
                  flexDirection: 'column',
                }}
              >
                {/* Error Alert */}
                {error && (
                  <Alert severity="error" sx={{my: 3}} onClose={() => setError(null)}>
                    {error}
                  </Alert>
                )}

                {renderStepContent()}

                {/* Navigation buttons */}
                <Box
                  sx={{
                    mt: 4,
                    display: 'flex',
                    justifyContent: visibleSteps.indexOf(currentStep) === 0 ? 'flex-end' : 'space-between',
                    gap: 2,
                  }}
                >
                  {visibleSteps.indexOf(currentStep) !== 0 && currentStep !== ApplicationCreateFlowStep.COMPLETE && (
                    <Button
                      variant="outlined"
                      onClick={handlePrevStep}
                      sx={{minWidth: 100}}
                      disabled={createApplication.isPending}
                    >
                      {t('common:actions.back')}
                    </Button>
                  )}

                  {currentStep !== ApplicationCreateFlowStep.COMPLETE && (
                    <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
                      {createFlow.isPending && <CircularProgress size={20} />}
                      <Button
                        data-testid="application-wizard-next-button"
                        variant="contained"
                        disabled={
                          !stepReady[currentStep] ||
                          createFlow.isPending ||
                          (currentStep === ApplicationCreateFlowStep.NAME && isOuLoading)
                        }
                        sx={{minWidth: 100}}
                        onClick={handleNextStep}
                      >
                        {visibleSteps.indexOf(currentStep) === visibleSteps.length - 1
                          ? t('common:actions.finish')
                          : t('common:actions.continue')}
                      </Button>
                    </Box>
                  )}
                </Box>
              </Box>
            </Box>
          </Box>
        </Box>
        {/* Right side - Preview (show from design step onwards, but hide on complete step) */}
        {currentStep !== ApplicationCreateFlowStep.STACK &&
          currentStep !== ApplicationCreateFlowStep.NAME &&
          currentStep !== ApplicationCreateFlowStep.ORGANIZATION_UNIT &&
          currentStep !== ApplicationCreateFlowStep.COMPLETE && (
            <Box sx={{flex: '0 0 50%', display: 'flex', flexDirection: 'column', p: 5}}>
              {isPreviewFlowLoading ? (
                <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1}}>
                  <CircularProgress />
                </Box>
              ) : (
                <GatePreview theme={selectedTheme} mock={previewMock} displayName={appName ?? undefined} />
              )}
            </Box>
          )}
      </Box>
    </Box>
  );
}
