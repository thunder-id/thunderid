// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGetApplications} from '@thunderid/configure-applications';
import {AuthenticatorTypes} from '@thunderid/configure-connections';
import type {AllowedOriginDraftRow} from '@thunderid/configure-settings';
import type {LayoutConfig, Theme} from '@thunderid/design';
import type {PropsWithChildren} from 'react';
import {useState, useMemo, useCallback} from 'react';
import ApplicationCreateContext, {type ApplicationCreateContextType} from './ApplicationCreateContext';
import type {BasicFlowDefinition} from '../../../flows/models/responses';
import {
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowStep,
  OrganizationUnitDefaultItem,
  type OrganizationUnitDefaultsSelection,
} from '../../models/application-create-flow';
import type {
  TechnologyApplicationTemplate,
  PlatformApplicationTemplate,
  ApplicationTemplate,
} from '../../models/application-templates';
import {McpClientTypes, type McpClientType} from '../../models/mcp-client';

/**
 * Props for the {@link ApplicationCreateProvider} component.
 *
 * @public
 */
export type ApplicationCreateProviderProps = PropsWithChildren;

/**
 * Default organization-unit-defaults selection: nothing selected until the Details step
 * resolves the chosen OU's configured defaults.
 *
 * @internal
 */
const NO_OU_DEFAULTS: OrganizationUnitDefaultsSelection = {
  [OrganizationUnitDefaultItem.SIGN_IN]: false,
  [OrganizationUnitDefaultItem.SIGN_UP]: false,
  [OrganizationUnitDefaultItem.RECOVERY]: false,
  [OrganizationUnitDefaultItem.SIGN_OUT]: false,
  [OrganizationUnitDefaultItem.THEME]: false,
  [OrganizationUnitDefaultItem.LAYOUT]: false,
};

/**
 * Initial state values for application creation
 *
 * @internal
 */
const INITIAL_STATE: {
  currentStep: ApplicationCreateFlowStep;
  appName: string;
  ouId: string;
  themeId: string | null;
  selectedTheme: Theme | null;
  layoutId: string | null;
  selectedLayout: LayoutConfig | null;
  appLogo: string | null;
  selectedColor: string;
  integrations: Record<string, boolean>;
  isEmailOtpMfaEnabled: boolean;
  isSmsOtpMfaEnabled: boolean;
  smsOtpSenderId: string;
  selectedAuthFlow: BasicFlowDefinition | null;
  signInApproach: ApplicationCreateFlowSignInApproach;
  registrationFlowId: string | null;
  isRegistrationFlowEnabled: boolean;
  recoveryFlowId: string | null;
  isRecoveryFlowEnabled: boolean;
  signOutFlowId: string | null;
  isSignOutFlowEnabled: boolean;
  redirectUris: string[];
  postLogoutRedirectUris: string[];
  corsOrigins: AllowedOriginDraftRow[];
  ouDefaults: OrganizationUnitDefaultsSelection;
  selectedTechnology: TechnologyApplicationTemplate | null;
  selectedPlatform: PlatformApplicationTemplate | null;
  mcpClientType: McpClientType;
  mcpRedirectUris: string[];
  hostingUrl: string;
  callbackUrlFromConfig: string;
  relyingPartyId: string;
  relyingPartyName: string;
  hasCompletedOnboarding: boolean;
  error: string | null;
} = {
  currentStep: ApplicationCreateFlowStep.ORGANIZATION_UNIT,
  appName: '',
  ouId: '',
  themeId: null,
  selectedTheme: null,
  layoutId: null,
  selectedLayout: null,
  appLogo: null,
  selectedColor: '#3B82F6',
  integrations: {
    [AuthenticatorTypes.CREDENTIALS_AUTH]: true,
  },
  isEmailOtpMfaEnabled: false,
  isSmsOtpMfaEnabled: false,
  smsOtpSenderId: '',
  selectedAuthFlow: null,
  signInApproach: ApplicationCreateFlowSignInApproach.REDIRECT_BASED as ApplicationCreateFlowSignInApproach,
  registrationFlowId: null,
  isRegistrationFlowEnabled: false,
  recoveryFlowId: null,
  isRecoveryFlowEnabled: false,
  signOutFlowId: null,
  isSignOutFlowEnabled: false,
  redirectUris: [],
  postLogoutRedirectUris: [],
  corsOrigins: [],
  ouDefaults: NO_OU_DEFAULTS,
  selectedTechnology: null,
  selectedPlatform: null,
  mcpClientType: McpClientTypes.USER_DELEGATED,
  mcpRedirectUris: [],
  hostingUrl: '',
  callbackUrlFromConfig: '',
  relyingPartyId: '',
  relyingPartyName: '',
  hasCompletedOnboarding: false,
  error: null,
};

/**
 * React context provider component that provides application creation state
 * to all child components.
 *
 * This component manages all the state needed across the multi-step onboarding process
 * for creating a new application. It provides state variables, setter functions, and
 * utility methods like toggleIntegration and reset.
 *
 * The provider creates utility methods for common operations such as toggling integrations
 * and resetting all state to initial values.
 *
 * @param props - The component props
 * @param props.children - React children to be wrapped with the application create context
 *
 * @returns JSX element that provides application creation context to children
 *
 * @example
 * ```tsx
 * import ApplicationCreateProvider from './ApplicationCreateProvider';
 * import ApplicationCreatePage from './ApplicationCreatePage';
 *
 * function App() {
 *   return (
 *     <ApplicationCreateProvider>
 *       <ApplicationCreatePage />
 *     </ApplicationCreateProvider>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function ApplicationCreateProvider({children}: ApplicationCreateProviderProps) {
  const {data: applicationsData} = useGetApplications({limit: 1, offset: 0});
  const [currentStep, setCurrentStep] = useState<ApplicationCreateFlowStep>(INITIAL_STATE.currentStep);
  const [appName, setAppName] = useState<string>(INITIAL_STATE.appName);
  const [ouId, setOuId] = useState<string>(INITIAL_STATE.ouId);
  const [themeId, setThemeId] = useState<string | null>(INITIAL_STATE.themeId);
  const [selectedTheme, setSelectedTheme] = useState<Theme | null>(INITIAL_STATE.selectedTheme);
  const [appLogo, setAppLogo] = useState<string | null>(INITIAL_STATE.appLogo);
  const [selectedColor, setSelectedColor] = useState<string>(INITIAL_STATE.selectedColor);
  const [integrations, setIntegrations] = useState<Record<string, boolean>>(INITIAL_STATE.integrations);
  const [isEmailOtpMfaEnabled, setIsEmailOtpMfaEnabled] = useState<boolean>(INITIAL_STATE.isEmailOtpMfaEnabled);
  const [isSmsOtpMfaEnabled, setIsSmsOtpMfaEnabled] = useState<boolean>(INITIAL_STATE.isSmsOtpMfaEnabled);
  const [smsOtpSenderId, setSmsOtpSenderId] = useState<string>(INITIAL_STATE.smsOtpSenderId);
  const [selectedAuthFlow, setSelectedAuthFlow] = useState<BasicFlowDefinition | null>(INITIAL_STATE.selectedAuthFlow);
  const [signInApproach, setSignInApproach] = useState<ApplicationCreateFlowSignInApproach>(
    INITIAL_STATE.signInApproach,
  );
  const [layoutId, setLayoutId] = useState<string | null>(INITIAL_STATE.layoutId);
  const [selectedLayout, setSelectedLayout] = useState<LayoutConfig | null>(INITIAL_STATE.selectedLayout);
  const [registrationFlowId, setRegistrationFlowId] = useState<string | null>(INITIAL_STATE.registrationFlowId);
  const [isRegistrationFlowEnabled, setIsRegistrationFlowEnabled] = useState<boolean>(
    INITIAL_STATE.isRegistrationFlowEnabled,
  );
  const [recoveryFlowId, setRecoveryFlowId] = useState<string | null>(INITIAL_STATE.recoveryFlowId);
  const [isRecoveryFlowEnabled, setIsRecoveryFlowEnabled] = useState<boolean>(INITIAL_STATE.isRecoveryFlowEnabled);
  const [signOutFlowId, setSignOutFlowId] = useState<string | null>(INITIAL_STATE.signOutFlowId);
  const [isSignOutFlowEnabled, setIsSignOutFlowEnabled] = useState<boolean>(INITIAL_STATE.isSignOutFlowEnabled);
  const [redirectUris, setRedirectUris] = useState<string[]>(INITIAL_STATE.redirectUris);
  const [postLogoutRedirectUris, setPostLogoutRedirectUris] = useState<string[]>(INITIAL_STATE.postLogoutRedirectUris);
  const [corsOrigins, setCorsOrigins] = useState<AllowedOriginDraftRow[]>(INITIAL_STATE.corsOrigins);
  const [ouDefaults, setOuDefaults] = useState<OrganizationUnitDefaultsSelection>(INITIAL_STATE.ouDefaults);
  const [selectedTechnology, setSelectedTechnology] = useState<TechnologyApplicationTemplate | null>(
    INITIAL_STATE.selectedTechnology,
  );
  const [selectedPlatform, setSelectedPlatform] = useState<PlatformApplicationTemplate | null>(
    INITIAL_STATE.selectedPlatform,
  );
  const [selectedTemplateConfig, setSelectedTemplateConfig] = useState<ApplicationTemplate | null>(null);
  const [mcpClientType, setMcpClientType] = useState<McpClientType>(INITIAL_STATE.mcpClientType);
  const [mcpRedirectUris, setMcpRedirectUris] = useState<string[]>(INITIAL_STATE.mcpRedirectUris);
  const [hostingUrl, setHostingUrl] = useState<string>(INITIAL_STATE.hostingUrl);
  const [callbackUrlFromConfig, setCallbackUrlFromConfig] = useState<string>(INITIAL_STATE.callbackUrlFromConfig);
  const [relyingPartyId, setRelyingPartyId] = useState<string>('');
  const [relyingPartyName, setRelyingPartyName] = useState<string>('');
  const [error, setError] = useState<string | null>(INITIAL_STATE.error);

  // A create failure goes stale the moment any wizard input changes, so any edit clears it.
  // JSON.stringify compares by value (integrations/redirectUris/etc. are rebuilt on every
  // change), and object-valued fields contribute only their id since they move in lockstep
  // with it. currentStep is deliberately excluded: stepping back and forth is navigation,
  // not a field edit, and shouldn't wipe an error the user still needs to read.
  const formFingerprint = JSON.stringify([
    appName,
    ouId,
    themeId,
    appLogo,
    selectedColor,
    integrations,
    isEmailOtpMfaEnabled,
    isSmsOtpMfaEnabled,
    smsOtpSenderId,
    selectedAuthFlow?.id ?? null,
    signInApproach,
    layoutId,
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
    selectedTechnology,
    selectedPlatform,
    selectedTemplateConfig?.id ?? null,
    mcpClientType,
    mcpRedirectUris,
    hostingUrl,
    callbackUrlFromConfig,
    relyingPartyId,
    relyingPartyName,
  ]);

  // Adjusted during render (React's documented pattern for state that must stay in sync with
  // another value) rather than in an effect, since a `useEffect` here would run a fully
  // committed render with the stale error still visible before clearing it a tick later.
  const [prevFormFingerprint, setPrevFormFingerprint] = useState(formFingerprint);
  if (formFingerprint !== prevFormFingerprint) {
    setPrevFormFingerprint(formFingerprint);
    setError(null);
  }

  // Derive onboarding completion status from fetched applications data
  const hasCompletedOnboarding = useMemo(() => {
    if (applicationsData?.applications && applicationsData.totalResults > 1) return true;
    if (applicationsData?.applications && applicationsData.totalResults === 0) return false;
    return INITIAL_STATE.hasCompletedOnboarding;
  }, [applicationsData?.applications, applicationsData?.totalResults]);

  const setHasCompletedOnboarding = useCallback(() => void 0, []);

  const toggleIntegration = useCallback((integrationId: string): void => {
    setIntegrations((prev) => ({
      ...prev,
      [integrationId]: !prev[integrationId],
    }));
  }, []);

  const reset = useCallback((): void => {
    setCurrentStep(INITIAL_STATE.currentStep);
    setAppName(INITIAL_STATE.appName);
    setOuId(INITIAL_STATE.ouId);
    setThemeId(INITIAL_STATE.themeId);
    setSelectedTheme(INITIAL_STATE.selectedTheme);
    setAppLogo(INITIAL_STATE.appLogo);
    setSelectedColor(INITIAL_STATE.selectedColor);
    setIntegrations(INITIAL_STATE.integrations);
    setIsEmailOtpMfaEnabled(INITIAL_STATE.isEmailOtpMfaEnabled);
    setIsSmsOtpMfaEnabled(INITIAL_STATE.isSmsOtpMfaEnabled);
    setSmsOtpSenderId(INITIAL_STATE.smsOtpSenderId);
    setSelectedAuthFlow(INITIAL_STATE.selectedAuthFlow);
    setSignInApproach(INITIAL_STATE.signInApproach);
    setLayoutId(INITIAL_STATE.layoutId);
    setSelectedLayout(INITIAL_STATE.selectedLayout);
    setRegistrationFlowId(INITIAL_STATE.registrationFlowId);
    setIsRegistrationFlowEnabled(INITIAL_STATE.isRegistrationFlowEnabled);
    setRecoveryFlowId(INITIAL_STATE.recoveryFlowId);
    setIsRecoveryFlowEnabled(INITIAL_STATE.isRecoveryFlowEnabled);
    setSignOutFlowId(INITIAL_STATE.signOutFlowId);
    setIsSignOutFlowEnabled(INITIAL_STATE.isSignOutFlowEnabled);
    setRedirectUris(INITIAL_STATE.redirectUris);
    setPostLogoutRedirectUris(INITIAL_STATE.postLogoutRedirectUris);
    setCorsOrigins(INITIAL_STATE.corsOrigins);
    setOuDefaults(INITIAL_STATE.ouDefaults);
    setSelectedTechnology(INITIAL_STATE.selectedTechnology);
    setSelectedPlatform(INITIAL_STATE.selectedPlatform);
    setMcpClientType(INITIAL_STATE.mcpClientType);
    setMcpRedirectUris(INITIAL_STATE.mcpRedirectUris);
    setHostingUrl(INITIAL_STATE.hostingUrl);
    setCallbackUrlFromConfig(INITIAL_STATE.callbackUrlFromConfig);
    setRelyingPartyId('');
    setRelyingPartyName('');
    setError(INITIAL_STATE.error);
  }, []);

  const contextValue: ApplicationCreateContextType = useMemo(
    () => ({
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
      selectedColor,
      setSelectedColor,
      integrations,
      setIntegrations,
      toggleIntegration,
      isEmailOtpMfaEnabled,
      setIsEmailOtpMfaEnabled,
      isSmsOtpMfaEnabled,
      setIsSmsOtpMfaEnabled,
      smsOtpSenderId,
      setSmsOtpSenderId,
      selectedAuthFlow,
      setSelectedAuthFlow,
      signInApproach,
      setSignInApproach,
      layoutId,
      setLayoutId,
      selectedLayout,
      setSelectedLayout,
      registrationFlowId,
      setRegistrationFlowId,
      isRegistrationFlowEnabled,
      setIsRegistrationFlowEnabled,
      recoveryFlowId,
      setRecoveryFlowId,
      isRecoveryFlowEnabled,
      setIsRecoveryFlowEnabled,
      signOutFlowId,
      setSignOutFlowId,
      isSignOutFlowEnabled,
      setIsSignOutFlowEnabled,
      redirectUris,
      setRedirectUris,
      postLogoutRedirectUris,
      setPostLogoutRedirectUris,
      corsOrigins,
      setCorsOrigins,
      ouDefaults,
      setOuDefaults,
      selectedTechnology,
      setSelectedTechnology,
      selectedPlatform,
      setSelectedPlatform,
      selectedTemplateConfig,
      setSelectedTemplateConfig,
      mcpClientType,
      setMcpClientType,
      mcpRedirectUris,
      setMcpRedirectUris,
      hostingUrl,
      setHostingUrl,
      callbackUrlFromConfig,
      setCallbackUrlFromConfig,
      relyingPartyId,
      setRelyingPartyId,
      relyingPartyName,
      setRelyingPartyName,
      hasCompletedOnboarding,
      setHasCompletedOnboarding,
      error,
      setError,
      reset,
    }),
    [
      currentStep,
      appName,
      ouId,
      themeId,
      selectedTheme,
      appLogo,
      selectedColor,
      integrations,
      toggleIntegration,
      isEmailOtpMfaEnabled,
      isSmsOtpMfaEnabled,
      smsOtpSenderId,
      selectedAuthFlow,
      signInApproach,
      layoutId,
      selectedLayout,
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
      selectedTechnology,
      selectedPlatform,
      selectedTemplateConfig,
      mcpClientType,
      mcpRedirectUris,
      hostingUrl,
      callbackUrlFromConfig,
      relyingPartyId,
      relyingPartyName,
      hasCompletedOnboarding,
      setHasCompletedOnboarding,
      error,
      reset,
    ],
  );

  return <ApplicationCreateContext.Provider value={contextValue}>{children}</ApplicationCreateContext.Provider>;
}
