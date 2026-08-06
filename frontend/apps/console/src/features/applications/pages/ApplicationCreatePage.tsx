// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {FullScreenCreationWizardLayout} from '@thunderid/components';
import {OAuth2GrantTypes, TokenEndpointAuthMethods, useGetApplications} from '@thunderid/configure-applications';
import type {Application, ApplicationType, OAuth2Config} from '@thunderid/configure-applications';
import {AuthenticatorTypes, IdentityProviderTypes, useIdentityProviders} from '@thunderid/configure-connections';
import {
  OrganizationUnitPickerScreen,
  useGetOrganizationUnit,
  useHasMultipleOUs,
} from '@thunderid/configure-organization-units';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {DefaultTheme, useGetTheme, type Theme} from '@thunderid/design';
import {useTemplateLiteralResolver} from '@thunderid/hooks';
import {useLogger} from '@thunderid/logger/react';
import type {EmbeddedFlowComponent} from '@thunderid/react';
import {Alert, Box, Button, CircularProgress, IconButton, Typography} from '@wso2/oxygen-ui';
import {ChevronLeft, ChevronRight, Home} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useState, useCallback, useEffect, useMemo} from 'react';
import {useTranslation} from 'react-i18next';
import {useLocation, useNavigate} from 'react-router';
import RouteConfig from '../../../configs/RouteConfig';
import useCreateFlow from '../../flows/api/useCreateFlow';
import useGetFlowById from '../../flows/api/useGetFlowById';
import type {BasicFlowDefinition} from '../../flows/models/responses';
import {resolveApplicationMeta, resolveTemplatesDeep} from '../../flows/utils/gatePreviewTransforms';
import generateFlowGraph from '../../flows/utils/generateFlowGraph';
import getFlowPromptComponentsSequence from '../../flows/utils/getFlowPromptComponentsSequence';
import useGetCorsConfig from '../../settings/api/useGetCorsConfig';
import useUpdateCorsConfig from '../../settings/api/useUpdateCorsConfig';
import useCreateApplication from '../api/useCreateApplication';
import ConfigureSecuritySettings from '../components/create-application/configure-security-settings/ConfigureSecuritySettings';
import ConfigureApplicationDetails from '../components/create-application/ConfigureApplicationDetails';
import ConfigureDesign from '../components/create-application/ConfigureDesign';
import ConfigureDetails from '../components/create-application/ConfigureDetails';
import ConfigureMcpClientType from '../components/create-application/mcp/ConfigureMcpClientType';
import ApplicationConstants from '../constants/application-constants';
import TemplateConstants from '../constants/template-constants';
import useApplicationCreate from '../contexts/ApplicationCreate/useApplicationCreate';
import {
  ApplicationCreateFlowConfiguration,
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowStep,
  OrganizationUnitDefaultItem,
} from '../models/application-create-flow';
import {PlatformApplicationTemplate, TechnologyApplicationTemplate} from '../models/application-templates';
import {McpClientTypes} from '../models/mcp-client';
import type {CreateApplicationRequest} from '../models/requests';
import getApplicationErrorMessage from '../utils/getApplicationErrorMessage';
import getConfigurationTypeFromTemplate from '../utils/getConfigurationTypeFromTemplate';
import isRedirectCapableTemplate from '../utils/isRedirectCapableTemplate';
import mergeCorsOrigins from '../utils/mergeCorsOrigins';
import resolveApplicationType from '../utils/resolveApplicationType';
import resolveCreationFlow from '../utils/resolveCreationFlow';
import GatePreview from '@/components/GatePreview/GatePreview';
import {VIEWPORT_WIDTHS, VIEWPORT_HEIGHTS} from '@/features/design/components/viewportConstants';

export default function ApplicationCreatePage(): JSX.Element {
  const {t} = useTranslation();
  const {resolveAll} = useTemplateLiteralResolver();

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
    layoutId,
    setLayoutId,
    setSelectedLayout,
    appLogo,
    setAppLogo,
    integrations,
    isEmailOtpMfaEnabled,
    isSmsOtpMfaEnabled,
    smsOtpSenderId,
    selectedAuthFlow,
    setSelectedAuthFlow,
    signInApproach,
    setSignInApproach,
    registrationFlowId,
    isRegistrationFlowEnabled,
    recoveryFlowId,
    isRecoveryFlowEnabled,
    signOutFlowId,
    isSignOutFlowEnabled,
    redirectUris,
    postLogoutRedirectUris,
    corsOrigins,
    ouDefaults,
    setOuDefaults,
    selectedTechnology,
    selectedPlatform,
    mcpClientType,
    setMcpClientType,
    mcpRedirectUris,
    setMcpRedirectUris,
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

  const isMcpClientTemplate = selectedTemplateConfig?.id === TemplateConstants.MCP_CLIENT_TEMPLATE_ID;

  const steps: Record<ApplicationCreateFlowStep, {label: string; order: number}> = useMemo(
    () => ({
      ORGANIZATION_UNIT: {label: t('applications:onboarding.steps.organizationUnit', 'Organization Unit'), order: 1},
      DETAILS: {label: t('applications:onboarding.steps.details'), order: 2},
      SECURITY: {label: t('applications:onboarding.steps.security'), order: 3},
      DESIGN: {label: t('applications:onboarding.steps.design'), order: 4},
      CONFIGURE: {
        label: t('applications:onboarding.steps.configure'),
        order: 6,
      },
      CLIENT_TYPE: {label: t('applications:onboarding.steps.configure'), order: 3},
      COMPLETE: {label: t('applications:onboarding.steps.complete'), order: 7},
    }),
    [t],
  );
  const navigate = useNavigate();
  const {pathname} = useLocation();
  const isWelcomeFlow = pathname.startsWith('/welcome');
  const logger = useLogger('ApplicationCreatePage');
  const createApplication = useCreateApplication();
  const {data: userTypesData} = useGetUserTypes();
  const {data: applicationsData} = useGetApplications({limit: 100});
  const {data: corsConfigData} = useGetCorsConfig();
  const updateCorsConfig = useUpdateCorsConfig();
  const {hasMultipleOUs, isLoading: isOuLoading, ouList} = useHasMultipleOUs();

  const handleBackToTemplates = (): void => {
    void navigate(isWelcomeFlow ? '/welcome/get-started/applications/types' : '/applications/types');
  };

  // The organization unit whose defaults are offered on the Details step: the picked one when
  // there are multiple, or the deployment's sole one otherwise.
  const resolvedOuId: string | undefined = hasMultipleOUs ? ouId || undefined : ouList[0]?.id;
  const {data: resolvedOrganizationUnit} = useGetOrganizationUnit(resolvedOuId, Boolean(resolvedOuId));

  // When the Details step snapshots the organization unit's theme, the preview should reflect
  // that actual theme rather than staying unconfigured (the Design step, which normally seeds it,
  // doesn't run in that case).
  const ouThemeId = ouDefaults[OrganizationUnitDefaultItem.THEME] ? (resolvedOrganizationUnit?.themeId ?? '') : '';
  const {data: ouThemeDetails} = useGetTheme(ouThemeId);
  // Until a real theme is resolved (OU snapshot still loading, or the Design step hasn't run
  // yet), the preview falls back to the same DefaultTheme the actual gate uses (see
  // apps/gate/src/hocs/withTheme.tsx) rather than sitting on a spinner or unstyled text.
  const previewTheme: Theme | undefined =
    (ouDefaults[OrganizationUnitDefaultItem.THEME] ? ouThemeDetails?.theme : selectedTheme) ?? undefined;

  // Client ids already in use, so the wallet step can flag a duplicate before submission.
  const existingClientIds = useMemo(
    (): string[] =>
      (applicationsData?.applications ?? [])
        .map((app) => app.clientId)
        .filter((clientId): clientId is string => Boolean(clientId)),
    [applicationsData],
  );

  // Application names already in use, so the details step can flag a duplicate before submission.
  const existingAppNames = useMemo(
    (): string[] => (applicationsData?.applications ?? []).map((app) => app.name).filter(Boolean),
    [applicationsData],
  );

  const [selectedUserTypes, setSelectedUserTypes] = useState<string[]>([]);
  // Names the server rejected as duplicates (APP-1020) that weren't in the fetched existingAppNames
  // (e.g. beyond the pagination limit). Treated as additional existing names so the details step
  // flags them and blocks readiness until the name is edited, avoiding a resubmit-and-fail loop.
  const [rejectedAppNames, setRejectedAppNames] = useState<string[]>([]);

  const knownAppNames = useMemo(
    (): string[] => [...existingAppNames, ...rejectedAppNames],
    [existingAppNames, rejectedAppNames],
  );

  const createFlow = useCreateFlow();
  const {data: idpData} = useIdentityProviders();

  const hasEnabledIntegrations = useMemo((): boolean => Object.values(integrations).some(Boolean), [integrations]);

  const previewFlowId: string | undefined =
    !hasEnabledIntegrations && selectedAuthFlow ? selectedAuthFlow.id : undefined;
  const {data: previewFlow, isLoading: isPreviewFlowLoading} = useGetFlowById(previewFlowId, Boolean(previewFlowId));

  // The raw graph both preview sources below walk carries unresolved `{{ meta(application.*) }}` /
  // `{{ t(...) }}` placeholders (the app isn't created yet, so there's no persisted Application to
  // resolve them against), so they're resolved here from the wizard's own in-progress details,
  // mirroring SimulationStepPreview's themed preview.
  const previewApplicationMeta = useMemo(
    (): Application => ({name: appName, logoUrl: appLogo ?? undefined}) as unknown as Application,
    [appName, appLogo],
  );

  // The wizard's own generated preview, as a sequence of screens: the primary sign-in screen, then
  // whatever screen(s) sequentially follow it (an MFA challenge, a magic-link "check your email"
  // wait screen, etc). Walks the exact same flow generateFlowGraph would submit on Continue — via
  // the same getFlowPromptComponentsSequence walker the pre-configured-flow case below uses — so
  // this preview can never drift from what toggling an auth method actually produces.
  const generatedPreviewScreens = useMemo((): EmbeddedFlowComponent[][] => {
    const availableIntegrations = idpData ?? [];
    const googleProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GOOGLE);
    const githubProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GITHUB);

    const generatedFlow = generateFlowGraph({
      appName,
      hasCredentialsAuth: integrations[AuthenticatorTypes.CREDENTIALS_AUTH] ?? false,
      hasPasskey: integrations[AuthenticatorTypes.PASSKEY] ?? false,
      hasMagicLink: integrations[AuthenticatorTypes.MAGIC_LINK] ?? false,
      googleIdpId: integrations[googleProvider?.id ?? ''] ? googleProvider?.id : undefined,
      githubIdpId: integrations[githubProvider?.id ?? ''] ? githubProvider?.id : undefined,
      hasSmsOtp: integrations['sms-otp'] ?? false,
      relyingPartyId: relyingPartyId || window.location.hostname,
      relyingPartyName: relyingPartyName || appName,
      hasEmailOtpMfa: isEmailOtpMfaEnabled,
      hasSmsOtpMfa: isSmsOtpMfaEnabled,
      smsOtpSenderId,
    });

    const screens = getFlowPromptComponentsSequence(generatedFlow) ?? [[]];
    return screens.map(
      (screen) =>
        resolveTemplatesDeep(
          resolveApplicationMeta(screen, previewApplicationMeta),
          (raw: string) => resolveAll(raw, {t}) ?? raw,
        ) as EmbeddedFlowComponent[],
    );
  }, [
    appName,
    integrations,
    idpData,
    relyingPartyId,
    relyingPartyName,
    isEmailOtpMfaEnabled,
    isSmsOtpMfaEnabled,
    smsOtpSenderId,
    previewApplicationMeta,
    resolveAll,
    t,
  ]);

  // A manually-selected pre-configured flow's own screens, walked directly from its graph — may
  // have any number of sequential screens (e.g. a hand-built multi-step flow).
  const externalFlowScreens = useMemo((): EmbeddedFlowComponent[][] | null => {
    const screens = getFlowPromptComponentsSequence(previewFlow);
    if (!screens) return screens;
    return screens.map(
      (screen) =>
        resolveTemplatesDeep(
          resolveApplicationMeta(screen, previewApplicationMeta),
          (raw: string) => resolveAll(raw, {t}) ?? raw,
        ) as EmbeddedFlowComponent[],
    );
  }, [previewFlow, previewApplicationMeta, resolveAll, t]);

  const allPreviewScreens: EmbeddedFlowComponent[][] = (hasEnabledIntegrations || !selectedAuthFlow
    ? generatedPreviewScreens
    : externalFlowScreens) ?? [[]];
  const totalPreviewSteps = allPreviewScreens.length;

  // Which preview screen is shown. Reset to the first screen on every wizard-step change AND
  // whenever the preview's underlying source changes (a different flow selected, or the toggled
  // integrations change) by adjusting state during render (React's recommended alternative to an
  // effect for this) rather than committing a stale, now out-of-range index first.
  const [previewStepIndex, setPreviewStepIndex] = useState(0);
  const [previewScreenStep, setPreviewScreenStep] = useState(currentStep);
  const previewSourceKey = `${selectedAuthFlow?.id ?? ''}|${JSON.stringify(integrations)}`;
  const [previewScreenSourceKey, setPreviewScreenSourceKey] = useState(previewSourceKey);
  if (currentStep !== previewScreenStep || previewSourceKey !== previewScreenSourceKey) {
    setPreviewScreenStep(currentStep);
    setPreviewScreenSourceKey(previewSourceKey);
    setPreviewStepIndex(0);
  }

  const activePreviewMock = allPreviewScreens[previewStepIndex] ?? [];

  const [stepReady, setStepReady] = useState<Record<ApplicationCreateFlowStep, boolean>>({
    ORGANIZATION_UNIT: false,
    DETAILS: false,
    SECURITY: true,
    DESIGN: true,
    CONFIGURE: true,
    CLIENT_TYPE: false,
    COMPLETE: true,
  });

  const [walletClientId, setWalletClientId] = useState<string>('');

  // The template is chosen on the standalone selection page before this wizard mounts. If the
  // wizard is reached directly (e.g. a bookmarked URL) with no template, send the user back to it.
  useEffect((): void => {
    if (selectedTemplateConfig) return;
    void navigate(isWelcomeFlow ? '/welcome/get-started/applications/types' : '/applications/types');
  }, [selectedTemplateConfig, isWelcomeFlow, navigate]);

  // Derive the OAuth config from the selected template's defaults, mirroring what the removed
  // in-wizard template step used to seed on mount.
  const oauthConfig = useMemo<OAuth2Config | null>(() => {
    if (!selectedTemplateConfig) return null;

    const oauthInboundConfig: OAuth2Config = selectedTemplateConfig.defaults?.inboundAuthConfig?.[0]?.config ?? {
      publicClient: false,
      pkceRequired: false,
      grantTypes: [],
      responseTypes: [],
      redirectUris: [],
      tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
    };

    return {
      publicClient: oauthInboundConfig.publicClient,
      pkceRequired: oauthInboundConfig.pkceRequired,
      grantTypes: [...oauthInboundConfig.grantTypes],
      responseTypes: [...(oauthInboundConfig.responseTypes ?? [])],
      redirectUris: oauthInboundConfig.redirectUris ? [...oauthInboundConfig.redirectUris] : [],
      tokenEndpointAuthMethod: oauthInboundConfig.tokenEndpointAuthMethod,
      scopes: ['openid', 'profile', 'email'],
    };
  }, [selectedTemplateConfig]);

  const effectiveOauthConfig = useMemo(() => {
    if (!oauthConfig) return oauthConfig;
    let config: OAuth2Config = callbackUrlFromConfig
      ? {...oauthConfig, redirectUris: [callbackUrlFromConfig]}
      : oauthConfig;
    if (walletClientId.trim()) {
      config = {...config, clientId: walletClientId.trim()};
    }
    // Merge in any additional redirect / post-logout redirect URIs entered on the Configuration
    // step, on top of the template-derived defaults above.
    const extraRedirectUris = redirectUris.map((uri) => uri.trim()).filter(Boolean);
    if (extraRedirectUris.length > 0) {
      config = {
        ...config,
        redirectUris: Array.from(new Set([...(config.redirectUris ?? []), ...extraRedirectUris])),
      };
    }
    const validPostLogoutRedirectUris = postLogoutRedirectUris.map((uri) => uri.trim()).filter(Boolean);
    if (validPostLogoutRedirectUris.length > 0) {
      config = {...config, postLogoutRedirectUris: validPostLogoutRedirectUris};
    }
    return config;
  }, [oauthConfig, callbackUrlFromConfig, walletClientId, redirectUris, postLogoutRedirectUris]);

  const creationFlow = useMemo(() => resolveCreationFlow(selectedTemplateConfig), [selectedTemplateConfig]);

  // The organization unit is the wizard's first step whenever there's a choice to make. Single-OU
  // deployments never need it, so once that's known, skip straight past it.
  useEffect((): void => {
    if (isOuLoading || hasMultipleOUs || currentStep !== ApplicationCreateFlowStep.ORGANIZATION_UNIT) return;
    const next = creationFlow.steps[creationFlow.steps.indexOf(ApplicationCreateFlowStep.ORGANIZATION_UNIT) + 1];
    if (next) setCurrentStep(next);
  }, [isOuLoading, hasMultipleOUs, currentStep, creationFlow, setCurrentStep]);

  // The canonical application type, resolved once from the template, falling back to the OAuth
  // profile for legacy/custom templates without an explicit type.
  const resolvedApplicationType = useMemo<ApplicationType>(
    () => resolveApplicationType(selectedTemplateConfig?.type, oauthConfig ?? undefined),
    [selectedTemplateConfig, oauthConfig],
  );

  // Whether this template's flow ever presents a hosted sign-in screen at all. Templates without
  // a SECURITY step (e.g. machine-to-machine backends) never render one, so there's nothing for
  // the wizard's sign-in preview to reflect.
  const hasSecurityStep = useMemo(
    (): boolean => creationFlow.steps.includes(ApplicationCreateFlowStep.SECURITY),
    [creationFlow],
  );

  // Whether this template's flow ever presents a Design step at all. Templates without one (e.g.
  // machine-to-machine backends) have no theme/layout of their own, so inheriting the organization
  // unit's design defaults wouldn't apply to anything.
  const hasDesignStep = useMemo(
    (): boolean => creationFlow.steps.includes(ApplicationCreateFlowStep.DESIGN),
    [creationFlow],
  );

  // Browser-based SPAs are public clients that must use the redirect-based flow, so the
  // embedded (native) sign-in approach is not offered for them. Native mobile apps (both the
  // generic Mobile platform template and the individual Android/iOS/Flutter technology
  // templates) and digital wallets are also public clients but legitimately use app-native
  // flows, so they are excluded from this rule.
  const isBrowserSpaTemplate = useMemo((): boolean => {
    if (
      selectedPlatform === PlatformApplicationTemplate.MOBILE ||
      selectedPlatform === PlatformApplicationTemplate.WALLET ||
      selectedTechnology === TechnologyApplicationTemplate.ANDROID ||
      selectedTechnology === TechnologyApplicationTemplate.IOS ||
      selectedTechnology === TechnologyApplicationTemplate.FLUTTER
    ) {
      return false;
    }
    const oauthConfig = selectedTemplateConfig?.defaults?.inboundAuthConfig?.find(
      (config) => config.type === 'oauth2',
    )?.config;
    return oauthConfig?.publicClient === true;
  }, [selectedTemplateConfig, selectedPlatform, selectedTechnology]);

  // Wallets are OID4VCI issuance flows that redirect to a hosted page; there's no native UI to
  // embed, so unlike mobile apps they get no sign-in approach choice at all.
  const isWalletTemplate = selectedPlatform === PlatformApplicationTemplate.WALLET;

  // Reset to the redirect-based approach if the embedded approach isn't allowed but is currently
  // selected (e.g. after switching away from a BYOUI-capable template).
  useEffect((): void => {
    if (isBrowserSpaTemplate && signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED) {
      setSignInApproach(ApplicationCreateFlowSignInApproach.REDIRECT_BASED);
    }
  }, [isBrowserSpaTemplate, signInApproach, setSignInApproach]);

  // Whether the template needs any of the Configure step's fields (hosting/callback URL, deep
  // link, passkey relying party). Embedded (native) sign-in doesn't redirect to hosted pages, so
  // none of the redirect/callback-oriented configuration applies to it — only passkey relying
  // party details still apply, since those are required regardless of how the user reaches
  // sign-in. This must mirror ConfigureDetails' own effectiveConfigurationType/
  // effectiveIsRedirectCapable so the step is skipped exactly when that step would render nothing.
  const needsConfigureFields = useMemo((): boolean => {
    const isPasskeyEnabled = !selectedAuthFlow && (integrations[AuthenticatorTypes.PASSKEY] ?? false);
    if (signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED) {
      return isPasskeyEnabled;
    }
    return (
      getConfigurationTypeFromTemplate(selectedTemplateConfig) !== ApplicationCreateFlowConfiguration.NONE ||
      isRedirectCapableTemplate(selectedTemplateConfig) ||
      isPasskeyEnabled
    );
  }, [selectedTemplateConfig, integrations, selectedAuthFlow, signInApproach]);

  // Browser SPAs are redirect-only (see isBrowserSpaTemplate above) and wallets are redirect-only
  // for a different reason (see isWalletTemplate above), so neither has anything to choose between
  // hosted pages and a custom UI — the Design step shows the picker for every other template.
  const showApproachSection = !isBrowserSpaTemplate && !isWalletTemplate;

  // The Configure step shows whenever the template needs configuring (hosting/callback URL, deep
  // link, passkey relying party), independent of the sign-in approach picker, which now lives on
  // the Design step.
  const showConfigureStep = needsConfigureFields;

  const visibleSteps = useMemo((): ApplicationCreateFlowStep[] => {
    return creationFlow.steps.filter((step) => {
      // Single-OU deployments have no organization unit choice to make.
      if (step === ApplicationCreateFlowStep.ORGANIZATION_UNIT) return hasMultipleOUs;
      if (step === ApplicationCreateFlowStep.CONFIGURE) return showConfigureStep;
      // Nothing left to show once the Sign In section is snapshotted from the organization
      // unit's default — Sign Up/Recovery/Sign Out live elsewhere and aren't configured here.
      if (step === ApplicationCreateFlowStep.SECURITY) return !ouDefaults[OrganizationUnitDefaultItem.SIGN_IN];
      // The Design step also hosts the sign-in approach picker, so it stays visible whenever
      // that picker is offered, regardless of the organization unit's theme/layout defaults.
      // Otherwise, nothing is left to show once every design default the organization unit
      // actually has (theme and/or layout) is snapshotted. An item the unit has no value for was
      // never available to opt into, so it shouldn't keep this step visible.
      if (step === ApplicationCreateFlowStep.DESIGN) {
        // Wallets have no approach picker but still need a theme for their hosted issuance pages,
        // so they also always keep this step regardless of the organization unit's design defaults.
        if (showApproachSection || isWalletTemplate) return true;
        const availableDesignItems = [OrganizationUnitDefaultItem.THEME, OrganizationUnitDefaultItem.LAYOUT].filter(
          (item) =>
            item === OrganizationUnitDefaultItem.THEME
              ? Boolean(resolvedOrganizationUnit?.themeId)
              : Boolean(resolvedOrganizationUnit?.layoutId),
        );
        return !(availableDesignItems.length > 0 && availableDesignItems.every((item) => ouDefaults[item]));
      }
      // COMPLETE is never rendered as a wizard step: the client secret is shown as a popup on the
      // created application's detail page instead, so it's filtered out of visibleSteps here.
      if (step === ApplicationCreateFlowStep.COMPLETE) return false;
      return true;
    });
  }, [
    creationFlow,
    hasMultipleOUs,
    showConfigureStep,
    showApproachSection,
    isWalletTemplate,
    ouDefaults,
    resolvedOrganizationUnit,
  ]);

  const handleClose = (): void => {
    void navigate(isWelcomeFlow ? '/home' : '/applications');
  };

  const handleLogoSelect = (logoUrl: string): void => {
    setAppLogo(logoUrl);
  };

  // Both of these feed the create payload but live in page state rather than the provider, so the
  // provider's form fingerprint doesn't cover them. Clear a stale create error here instead.
  const handleUserTypesChange = (userTypes: string[]): void => {
    setError(null);
    setSelectedUserTypes(userTypes);
  };

  const handleWalletClientIdChange = (clientId: string): void => {
    setError(null);
    setWalletClientId(clientId);
  };

  const handleCreateApplication = (skipOAuthConfig = false, overrideFlowId?: string): void => {
    setError(null);

    const includesSecurity = hasSecurityStep;
    const includesDesign = creationFlow.steps.includes(ApplicationCreateFlowStep.DESIGN);

    // Each organization-unit-defaultable item resolves to either a snapshot of the organization
    // unit's current value (when its Details-step toggle is on) or the value built up elsewhere
    // in the wizard.
    const finalAuthFlowId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.SIGN_IN]
      ? (resolvedOrganizationUnit?.authFlowId ?? undefined)
      : (overrideFlowId ?? selectedAuthFlow?.id);
    const finalRegistrationFlowId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.SIGN_UP]
      ? (resolvedOrganizationUnit?.registrationFlowId ?? undefined)
      : (registrationFlowId ?? undefined);
    const finalIsRegistrationFlowEnabled: boolean = ouDefaults[OrganizationUnitDefaultItem.SIGN_UP]
      ? Boolean(resolvedOrganizationUnit?.isRegistrationFlowEnabled)
      : isRegistrationFlowEnabled;
    const finalRecoveryFlowId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.RECOVERY]
      ? (resolvedOrganizationUnit?.recoveryFlowId ?? undefined)
      : (recoveryFlowId ?? undefined);
    const finalIsRecoveryFlowEnabled: boolean = ouDefaults[OrganizationUnitDefaultItem.RECOVERY]
      ? Boolean(resolvedOrganizationUnit?.isRecoveryFlowEnabled)
      : isRecoveryFlowEnabled;
    const finalSignOutFlowId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.SIGN_OUT]
      ? (resolvedOrganizationUnit?.signOutFlowId ?? undefined)
      : (signOutFlowId ?? undefined);
    const finalIsSignOutFlowEnabled: boolean = ouDefaults[OrganizationUnitDefaultItem.SIGN_OUT]
      ? Boolean(resolvedOrganizationUnit?.isSignOutFlowEnabled)
      : isSignOutFlowEnabled;
    const finalThemeId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.THEME]
      ? (resolvedOrganizationUnit?.themeId ?? undefined)
      : (themeId ?? undefined);
    const finalLayoutId: string | undefined = ouDefaults[OrganizationUnitDefaultItem.LAYOUT]
      ? (resolvedOrganizationUnit?.layoutId ?? undefined)
      : (layoutId ?? undefined);

    // authFlowId is only required when the flow has user-facing sign-in steps
    if (includesSecurity && !finalAuthFlowId) {
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
      templateId && signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED
        ? `${templateId}${TemplateConstants.EMBEDDED_SUFFIX}`
        : templateId;

    // The canonical application type resolved above. The MCP client template can resolve to two
    // types, so the machine-to-machine selection overrides it.
    const applicationType: ApplicationType =
      isMcpClientTemplate && mcpClientType === McpClientTypes.M2M ? 'm2m' : resolvedApplicationType;

    // The mcp-client template branches the oauth2 config off the template-seeded config (never
    // rebuilt from scratch): user-delegated keeps the seeded authorization_code/refresh_token/PKCE/
    // public-client shape and adds the collected redirect URIs; machine-to-machine overrides it to a
    // confidential client_credentials shape with no redirect URIs.
    const mcpOAuth2Config: OAuth2Config | null =
      isMcpClientTemplate && oauthConfig
        ? mcpClientType === McpClientTypes.M2M
          ? {
              ...oauthConfig,
              grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS],
              responseTypes: [],
              redirectUris: [],
              pkceRequired: false,
              publicClient: false,
              tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
            }
          : {...oauthConfig, redirectUris: mcpRedirectUris.filter((uri) => uri.trim() !== '')}
        : null;

    const applicationData: CreateApplicationRequest = {
      name: appName,
      ...(hostingUrl && {url: hostingUrl}),
      ...(finalAuthFlowId && {authFlowId: finalAuthFlowId}),
      ...(effectiveOuId && {ouId: effectiveOuId}),
      ...(applicationType && {type: applicationType}),
      ...(finalTemplateId && {template: finalTemplateId}),
      ...(includesDesign && {
        logoUrl: appLogo ?? undefined,
        ...(finalThemeId && {themeId: finalThemeId}),
        ...(finalLayoutId && {layoutId: finalLayoutId}),
      }),
      ...(includesSecurity && {
        userAttributes: isSmsOtpMfaEnabled
          ? ['given_name', 'family_name', 'email', 'phone_number', 'groups']
          : ['given_name', 'family_name', 'email', 'groups'],
        isRegistrationFlowEnabled: finalIsRegistrationFlowEnabled,
        ...(finalRegistrationFlowId && {registrationFlowId: finalRegistrationFlowId}),
        isRecoveryFlowEnabled: finalIsRecoveryFlowEnabled,
        ...(finalRecoveryFlowId && {recoveryFlowId: finalRecoveryFlowId}),
      }),
      ...(isMcpClientTemplate &&
        mcpClientType === McpClientTypes.USER_DELEGATED && {
          userAttributes: ['given_name', 'family_name', 'email', 'groups'],
          isRegistrationFlowEnabled: true,
        }),
      ...(finalSignOutFlowId && {
        signOutFlowId: finalSignOutFlowId,
        isSignOutFlowEnabled: finalIsSignOutFlowEnabled,
      }),
      ...(allowedUserTypes && {allowedUserTypes}),
      ...(!skipOAuthConfig && {
        inboundAuthConfig: [{type: 'oauth2', config: mcpOAuth2Config ?? effectiveOauthConfig}],
      }),
    };

    createApplication.mutate(applicationData, {
      onSuccess: (createdApp: Application): void => {
        // CORS is a deployment-wide setting, not per-application, so origins entered on the
        // Configuration step are merged into the deployment's allow-list here rather than sent as
        // part of the application payload above. This is independent of application creation
        // succeeding, so it neither blocks nor is blocked by the navigation below.
        const validCorsAdditions = corsOrigins.map((origin) => origin.trim()).filter(Boolean);
        if (selectedTemplateConfig?.capabilities?.cors && validCorsAdditions.length > 0) {
          updateCorsConfig.mutate({
            data: mergeCorsOrigins(
              corsConfigData?.writable.allowedOrigins ?? [],
              corsConfigData?.readOnly.allowedOrigins ?? [],
              validCorsAdditions,
            ),
          });
        }

        // The mcp-client template has no completion step of its own, so it always goes straight
        // to the created application's detail page.
        if (isMcpClientTemplate) {
          (async () => {
            await navigate(RouteConfig.applications.detail(createdApp.id));
          })().catch((_error: unknown) => {
            logger.error('Failed to navigate to application details', {error: _error, applicationId: createdApp.id});
          });
          return;
        }

        const oauth2Config = createdApp.inboundAuthConfig?.find((config) => config.type === 'oauth2');
        const clientId = oauth2Config?.config?.clientId;
        const clientSecret = oauth2Config?.config?.clientSecret;
        const {flowSecret} = createdApp;
        // Backend / server-side apps receive a Flow Secret (top-level) even when they have no
        // OAuth client secret, so surface the save-secret popup for either credential.
        const hasSecret = Boolean(clientSecret) || Boolean(flowSecret);

        (async () => {
          if (hasSecret) {
            await navigate(RouteConfig.applications.detail(createdApp.id), {
              state: {justCreatedSecret: {appName, clientId, clientSecret, flowSecret}},
            });
          } else {
            await navigate(RouteConfig.applications.detail(createdApp.id));
          }
        })().catch((_error: unknown) => {
          logger.error('Failed to navigate to application details', {error: _error, applicationId: createdApp.id});
        });
      },
      onError: (err: Error) => {
        // The client-side pre-check can miss names beyond the fetched list; when the backend rejects
        // a name as a duplicate, record it so the details step flags it and stays un-ready until the
        // name is edited (preventing a resubmit-and-fail loop), then return there.
        // getApplicationErrorMessage resolves the errors.APP-1020 message for this case (and the
        // generic message otherwise).
        const errorCode = (err as {response?: {data?: {code?: string}}})?.response?.data?.code;
        if (errorCode === ApplicationConstants.DUPLICATE_APP_NAME_ERROR_CODE) {
          setRejectedAppNames((prev) => (prev.includes(appName) ? prev : [...prev, appName]));
          setCurrentStep(ApplicationCreateFlowStep.DETAILS);
        }
        setError(
          getApplicationErrorMessage(
            err,
            (key, options) => t(key.includes(':') ? key : `applications:${key}`, options),
            'create.error',
            'Failed to create application. Please try again.',
          ),
        );
      },
    });
  };

  const ensureFlowAndCreateApplication = (skipOAuthConfig = false): void => {
    const hasEnabledIntegrations = Object.values(integrations).some((v) => v);

    // A selected flow only short-circuits generation when it's a genuine manual pick (no
    // integrations enabled). Otherwise it's an auto-match riding along with active toggles, and
    // must still be regenerated below so MFA settings aren't silently dropped.
    if (selectedAuthFlow && !hasEnabledIntegrations) {
      handleCreateApplication(skipOAuthConfig);
      return;
    }

    if (hasEnabledIntegrations) {
      const availableIntegrations = idpData ?? [];
      const googleProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GOOGLE);
      const githubProvider = availableIntegrations.find((idp) => idp.type === IdentityProviderTypes.GITHUB);

      const generatedFlowRequest = generateFlowGraph({
        appName,
        hasCredentialsAuth: integrations[AuthenticatorTypes.CREDENTIALS_AUTH] ?? false,
        hasPasskey: integrations[AuthenticatorTypes.PASSKEY] ?? false,
        hasMagicLink: integrations[AuthenticatorTypes.MAGIC_LINK] ?? false,
        googleIdpId: integrations[googleProvider?.id ?? ''] ? googleProvider?.id : undefined,
        githubIdpId: integrations[githubProvider?.id ?? ''] ? githubProvider?.id : undefined,
        hasSmsOtp: integrations['sms-otp'] ?? false,
        relyingPartyId: relyingPartyId || window.location.hostname,
        relyingPartyName: relyingPartyName || appName,
        hasEmailOtpMfa: isEmailOtpMfaEnabled,
        hasSmsOtpMfa: isSmsOtpMfaEnabled,
        smsOtpSenderId,
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
    // DETAILS has a special wait condition for OU loading
    if (currentStep === ApplicationCreateFlowStep.DETAILS && isOuLoading) return;

    const idx = visibleSteps.indexOf(currentStep);
    const next = visibleSteps[idx + 1];

    if (next === undefined) {
      // No more visible steps → create the application
      const skipOAuth = signInApproach === ApplicationCreateFlowSignInApproach.EMBEDDED;

      if (hasSecurityStep && !ouDefaults[OrganizationUnitDefaultItem.SIGN_IN]) {
        // user-facing flow → may need to generate the auth flow from integrations
        ensureFlowAndCreateApplication(skipOAuth);
      } else {
        // OU-default sign-in flow or m2m flow → no integrations; nothing to generate
        handleCreateApplication(skipOAuth);
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

  const handleDetailsStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.DETAILS, isReady);
    },
    [handleStepReadyChange],
  );

  const handleDesignStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.DESIGN, isReady);
    },
    [handleStepReadyChange],
  );

  const handleSecurityStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.SECURITY, isReady);
    },
    [handleStepReadyChange],
  );

  const handleConfigureStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.CONFIGURE, isReady);
    },
    [handleStepReadyChange],
  );

  const handleClientTypeStepReadyChange = useCallback(
    (isReady: boolean): void => {
      handleStepReadyChange(ApplicationCreateFlowStep.CLIENT_TYPE, isReady);
    },
    [handleStepReadyChange],
  );

  const renderStepContent = (): JSX.Element | null => {
    switch (currentStep) {
      case ApplicationCreateFlowStep.DETAILS:
        return (
          <ConfigureApplicationDetails
            hasMultipleOUs={hasMultipleOUs}
            selectedOuId={ouId}
            onChangeOu={() => setCurrentStep(ApplicationCreateFlowStep.ORGANIZATION_UNIT)}
            resolvedOuId={resolvedOuId}
            appName={appName}
            onAppNameChange={setAppName}
            appLogo={appLogo}
            onLogoSelect={handleLogoSelect}
            ouDefaults={ouDefaults}
            onOuDefaultsChange={setOuDefaults}
            hasSecurityStep={hasSecurityStep}
            hasDesignStep={hasDesignStep}
            userTypes={userTypesData?.types ?? []}
            selectedUserTypes={selectedUserTypes}
            onUserTypesChange={handleUserTypesChange}
            onReadyChange={handleDetailsStepReadyChange}
            existingAppNames={knownAppNames}
          />
        );

      case ApplicationCreateFlowStep.CLIENT_TYPE:
        return (
          <ConfigureMcpClientType
            selectedType={mcpClientType}
            onSelect={setMcpClientType}
            redirectUris={mcpRedirectUris}
            onRedirectUrisChange={setMcpRedirectUris}
            onReadyChange={handleClientTypeStepReadyChange}
          />
        );

      case ApplicationCreateFlowStep.DESIGN:
        return (
          <ConfigureDesign
            themeId={themeId}
            layoutId={layoutId}
            onThemeSelect={(id, config) => {
              setThemeId(id);
              setSelectedTheme(config);
            }}
            onLayoutSelect={(id, config) => {
              setLayoutId(id);
              setSelectedLayout(config);
            }}
            onReadyChange={handleDesignStepReadyChange}
            selectedApproach={signInApproach}
            onApproachChange={setSignInApproach}
            allowEmbeddedApproach={!isBrowserSpaTemplate}
            showApproachSection={showApproachSection}
          />
        );

      case ApplicationCreateFlowStep.SECURITY:
        return <ConfigureSecuritySettings onReadyChange={handleSecurityStepReadyChange} />;

      case ApplicationCreateFlowStep.CONFIGURE:
        return (
          <ConfigureDetails
            technology={selectedTechnology}
            platform={selectedPlatform}
            onHostingUrlChange={setHostingUrl}
            onCallbackUrlChange={setCallbackUrlFromConfig}
            onClientIdChange={handleWalletClientIdChange}
            existingClientIds={existingClientIds}
            selectedApproach={signInApproach}
            onReadyChange={handleConfigureStepReadyChange}
          />
        );

      default:
        return null;
    }
  };

  const getStepProgress = (): number => {
    const totalSteps = visibleSteps.length;
    const currentIndex = visibleSteps.indexOf(currentStep);
    return ((currentIndex + 1) / totalSteps) * 100;
  };

  const getBreadcrumbSteps = (): ApplicationCreateFlowStep[] => {
    const idx = visibleSteps.indexOf(currentStep);
    if (idx < 0) return visibleSteps;
    return visibleSteps.slice(0, idx + 1);
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
          key: 'applications',
          label: t('navigation:pages.applications'),
          onClick: () => void navigate(RouteConfig.applications.list()),
        },
      ];

  // Which steps render the live sign-in preview panel is declared by the template itself
  // (`creationFlow.previewSteps`), not guessed here from the current step or platform — e.g. the
  // wallet template's CONFIGURE step has no hosted sign-in screen to preview (it just wires up
  // wallet client details and redirect URIs) and machine-to-machine templates never present a
  // hosted sign-in screen at all, so both simply omit the relevant steps from their own
  // `previewSteps` list.
  const hasNoPreview = !creationFlow.previewSteps.includes(currentStep);

  // The wizard preview has no viewport switcher of its own (see the `showToolbar={false}` below)
  // — the device frame it renders at is fixed per template via `previewDevice`, defaulting to
  // desktop when a template doesn't set it.
  const isMobilePreview = selectedTemplateConfig?.previewDevice === 'mobile';
  const previewViewport = isMobilePreview
    ? {width: VIEWPORT_WIDTHS.mobile, height: VIEWPORT_HEIGHTS.mobile}
    : undefined;

  const isFirstStep = visibleSteps.indexOf(currentStep) === 0;
  const isLastStep = visibleSteps.indexOf(currentStep) === visibleSteps.length - 1;

  // Switching the organization unit invalidates any previously inherited defaults: the new OU may
  // not offer the same defaultable items, and the Details step's reseeding effect only turns
  // items *on* for what's available on the new OU, it never turns stale ones back off.
  const handleOuIdChange = (newOuId: string): void => {
    setOuId(newOuId);
    setOuDefaults({
      [OrganizationUnitDefaultItem.SIGN_IN]: false,
      [OrganizationUnitDefaultItem.SIGN_UP]: false,
      [OrganizationUnitDefaultItem.RECOVERY]: false,
      [OrganizationUnitDefaultItem.SIGN_OUT]: false,
      [OrganizationUnitDefaultItem.THEME]: false,
      [OrganizationUnitDefaultItem.LAYOUT]: false,
    });
  };

  if (currentStep === ApplicationCreateFlowStep.ORGANIZATION_UNIT) {
    if (isOuLoading || !hasMultipleOUs) {
      return (
        <Box sx={{minHeight: '100vh', display: 'flex', alignItems: 'center', justifyContent: 'center'}}>
          <CircularProgress />
        </Box>
      );
    }

    return (
      <OrganizationUnitPickerScreen
        icon={<Home size={26} />}
        title={t('applications:onboarding.organizationUnit.title')}
        subtitle={t('applications:onboarding.organizationUnit.subtitle')}
        value={ouId}
        onChange={handleOuIdChange}
        onBack={ouId ? () => setCurrentStep(ApplicationCreateFlowStep.DETAILS) : handleBackToTemplates}
        onContinue={handleNextStep}
        backLabel={t('common:actions.back')}
        continueLabel={t('common:actions.continue')}
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
          label: steps[step].label,
          onClick: index < array.length - 1 ? () => setCurrentStep(step) : undefined,
        })),
      ]}
      preview={
        hasNoPreview ? undefined : isPreviewFlowLoading ? (
          <Box sx={{display: 'flex', justifyContent: 'center', alignItems: 'center', flex: 1}}>
            <CircularProgress />
          </Box>
        ) : (
          <Box sx={{flex: 1, minHeight: 0, display: 'flex', flexDirection: 'column'}}>
            <GatePreview
              theme={previewTheme}
              baseTheme={DefaultTheme as Theme}
              mock={activePreviewMock}
              displayName={appName ?? undefined}
              showToolbar={false}
              viewport={previewViewport}
              frameStyle={isMobilePreview ? 'phone' : 'browser'}
              footer={
                currentStep === ApplicationCreateFlowStep.SECURITY && totalPreviewSteps >= 2 ? (
                  <Box sx={{display: 'flex', alignItems: 'center', justifyContent: 'center', gap: 2, py: 1.5}}>
                    <IconButton
                      size="small"
                      disabled={previewStepIndex === 0}
                      onClick={() => setPreviewStepIndex((index) => Math.max(0, index - 1))}
                      aria-label={t('common:actions.previous')}
                      sx={{bgcolor: 'action.hover'}}
                    >
                      <ChevronLeft size={18} />
                    </IconButton>
                    <Typography variant="body2" color="text.secondary">
                      {t('applications:onboarding.preview.stepOf', 'Step {{n}} of {{total}}', {
                        n: previewStepIndex + 1,
                        total: totalPreviewSteps,
                      })}
                    </Typography>
                    <IconButton
                      size="small"
                      disabled={previewStepIndex === totalPreviewSteps - 1}
                      onClick={() => setPreviewStepIndex((index) => Math.min(totalPreviewSteps - 1, index + 1))}
                      aria-label={t('common:actions.next')}
                      sx={{
                        bgcolor: previewStepIndex === totalPreviewSteps - 1 ? 'action.hover' : 'primary.main',
                        color: previewStepIndex === totalPreviewSteps - 1 ? undefined : 'primary.contrastText',
                        '&:hover': {
                          bgcolor: previewStepIndex === totalPreviewSteps - 1 ? 'action.hover' : 'primary.dark',
                        },
                      }}
                    >
                      <ChevronRight size={18} />
                    </IconButton>
                  </Box>
                ) : undefined
              }
            />
          </Box>
        )
      }
      footer={
        <Box sx={{display: 'flex', justifyContent: isFirstStep ? 'flex-end' : 'space-between', gap: 2}}>
          {!isFirstStep && (
            <Button
              variant="outlined"
              onClick={handlePrevStep}
              sx={{minWidth: 100}}
              disabled={createApplication.isPending}
            >
              {t('common:actions.back')}
            </Button>
          )}

          <Box sx={{display: 'flex', alignItems: 'center', gap: 2}}>
            {createFlow.isPending && <CircularProgress size={20} />}
            <Button
              data-testid="application-wizard-next-button"
              variant="contained"
              disabled={!stepReady[currentStep] || createFlow.isPending}
              sx={{minWidth: 100}}
              onClick={handleNextStep}
            >
              {isLastStep ? t('common:actions.finish') : t('common:actions.continue')}
            </Button>
          </Box>
        </Box>
      }
    >
      {error && (
        <Alert severity="error" sx={{mb: 3}} onClose={() => setError(null)}>
          {error}
        </Alert>
      )}

      {renderStepContent()}
    </FullScreenCreationWizardLayout>
  );
}
