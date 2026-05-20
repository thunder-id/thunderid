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

// ── Plugin ──
export {default as ThunderIDPlugin} from './plugins/ThunderIDPlugin';
export type {ThunderIDPluginOptions} from './plugins/ThunderIDPlugin';

// ── Components ──
export {default as ThunderIDProvider} from './providers/ThunderIDProvider';

// ── Providers ──
export {default as BrandingProvider} from './providers/BrandingProvider';
export {default as FlowMetaProvider} from './providers/FlowMetaProvider';
export {default as FlowProvider} from './providers/FlowProvider';
export {default as I18nProvider} from './providers/I18nProvider';
export {default as OrganizationProvider} from './providers/OrganizationProvider';
export {default as ThemeProvider} from './providers/ThemeProvider';
export {default as UserProvider} from './providers/UserProvider';

// ── Composables ──
export {default as useThunderID} from './composables/useThunderID';
export {default as useBranding} from './composables/useBranding';
export {default as useFlow} from './composables/useFlow';
export {default as useFlowMeta} from './composables/useFlowMeta';
export {default as useI18n} from './composables/useI18n';
export {default as useOrganization} from './composables/useOrganization';
export {default as useTheme} from './composables/useTheme';
export {default as useUser} from './composables/useUser';
export {useOAuthCallback} from './composables/useOAuthCallback';
export type {UseOAuthCallbackOptions, OAuthCallbackPayload} from './composables/useOAuthCallback';
export {useOAuthCallback as useOAuthCallbackV2} from './composables/v2/useOAuthCallback';
export type {
  UseOAuthCallbackOptions as UseOAuthCallbackOptionsV2,
  OAuthCallbackPayload as OAuthCallbackPayloadV2,
} from './composables/v2/useOAuthCallback';

// ── Client ──
export {default as ThunderIDVueClient} from './ThunderIDVueClient';

// ── Keys ──
export {
  THUNDERID_KEY,
  BRANDING_KEY,
  FLOW_KEY,
  FLOW_META_KEY,
  I18N_KEY,
  ORGANIZATION_KEY,
  THEME_KEY,
  USER_KEY,
} from './keys';

// ── Models / Types ──
export type {ThunderIDVueConfig} from './models/config';
export type {
  ThunderIDContext,
  BrandingContextValue,
  FlowContextValue,
  FlowMessage,
  FlowMetaContextValue,
  FlowStep,
  I18nContextValue,
  OrganizationContextValue,
  ThemeContextValue,
  UserContextValue,
} from './models/contexts';

// ── UI Components — Primitives ──
export {default as Button} from './components/primitives/Button/Button';
export {default as Card} from './components/primitives/Card/Card';
export {default as Alert} from './components/primitives/Alert/Alert';
export {default as TextField} from './components/primitives/TextField/TextField';
export {default as PasswordField} from './components/primitives/PasswordField/PasswordField';
export {default as Select} from './components/primitives/Select/Select';
export type {SelectOption} from './components/primitives/Select/Select';
export {default as Checkbox} from './components/primitives/Checkbox/Checkbox';
export {default as DatePicker} from './components/primitives/DatePicker/DatePicker';
export {default as OtpField} from './components/primitives/OtpField/OtpField';
export {default as Typography} from './components/primitives/Typography/Typography';
export {default as Divider} from './components/primitives/Divider/Divider';
export {default as Logo} from './components/primitives/Logo/Logo';
export {default as Spinner} from './components/primitives/Spinner/Spinner';
export {
  UserIcon,
  EyeIcon,
  EyeOffIcon,
  ChevronDownIcon,
  CheckIcon,
  CircleAlertIcon,
  CircleCheckIcon,
  InfoIcon,
  TriangleAlertIcon,
  XIcon,
  PlusIcon,
  LogOutIcon,
  ArrowLeftRightIcon,
  BuildingIcon,
  GlobeIcon,
  PencilIcon,
} from './components/primitives/Icons';

// ── UI Components — Actions ──
export {default as SignInButton} from './components/actions/SignInButton';
export {default as BaseSignInButton} from './components/actions/BaseSignInButton';
export {default as SignOutButton} from './components/actions/SignOutButton';
export {default as BaseSignOutButton} from './components/actions/BaseSignOutButton';
export {default as SignUpButton} from './components/actions/SignUpButton';
export {default as BaseSignUpButton} from './components/actions/BaseSignUpButton';

// ── UI Components — Auth Flow ──
export {default as Callback} from './components/auth/Callback';
export {default as SignIn} from './components/auth/sign-in/SignIn';
export type {SignInRenderProps} from './components/auth/sign-in/SignIn';
export {default as BaseSignIn} from './components/auth/sign-in/BaseSignIn';
export type {BaseSignInRenderProps, BaseSignInProps} from './components/auth/sign-in/BaseSignIn';
export {default as SignUp} from './components/auth/sign-up/SignUp';
export type {SignUpRenderProps} from './components/auth/sign-up/SignUp';
export {default as BaseSignUp} from './components/auth/sign-up/BaseSignUp';
export type {BaseSignUpRenderProps, BaseSignUpProps} from './components/auth/sign-up/BaseSignUp';

// ── UI Components — Control ──
export {default as SignedIn} from './components/control/SignedIn';
export {default as SignedOut} from './components/control/SignedOut';
export {default as Loading} from './components/control/Loading';

// ── UI Components — Presentation ──
export {default as User} from './components/presentation/user/User';
export {default as Organization} from './components/presentation/organization/Organization';
export {default as UserProfile} from './components/presentation/user-profile/UserProfile';
export {default as BaseUserProfile} from './components/presentation/user-profile/BaseUserProfile';
export {default as UserDropdown} from './components/presentation/user-dropdown/UserDropdown';
export {default as BaseUserDropdown} from './components/presentation/user-dropdown/BaseUserDropdown';
export {default as AcceptInvite} from './components/presentation/accept-invite/AcceptInvite';
export type {AcceptInviteRenderProps} from './components/presentation/accept-invite/AcceptInvite';
export {default as BaseAcceptInvite} from './components/presentation/accept-invite/BaseAcceptInvite';
export type {
  BaseAcceptInviteRenderProps,
  BaseAcceptInviteProps,
} from './components/presentation/accept-invite/BaseAcceptInvite';
export {default as InviteUser} from './components/presentation/invite-user/InviteUser';
export type {InviteUserRenderProps} from './components/presentation/invite-user/InviteUser';
export {default as BaseInviteUser} from './components/presentation/invite-user/BaseInviteUser';
export type {
  BaseInviteUserRenderProps,
  BaseInviteUserProps,
} from './components/presentation/invite-user/BaseInviteUser';
export {default as OrganizationList} from './components/presentation/organization-list/OrganizationList';
export {default as BaseOrganizationList} from './components/presentation/organization-list/BaseOrganizationList';
export {default as OrganizationProfile} from './components/presentation/organization-profile/OrganizationProfile';
export {default as BaseOrganizationProfile} from './components/presentation/organization-profile/BaseOrganizationProfile';
export {default as OrganizationSwitcher} from './components/presentation/organization-switcher/OrganizationSwitcher';
export {default as BaseOrganizationSwitcher} from './components/presentation/organization-switcher/BaseOrganizationSwitcher';
export {default as CreateOrganization} from './components/presentation/create-organization/CreateOrganization';
export {default as BaseCreateOrganization} from './components/presentation/create-organization/BaseCreateOrganization';
export {default as LanguageSwitcher} from './components/presentation/language-switcher/LanguageSwitcher';
export {default as BaseLanguageSwitcher} from './components/presentation/language-switcher/BaseLanguageSwitcher';

// ── UI Components — Adapters ──
export {default as GoogleButton} from './components/adapters/GoogleButton';
export {default as GitHubButton} from './components/adapters/GitHubButton';
export {default as MicrosoftButton} from './components/adapters/MicrosoftButton';
export {default as FacebookButton} from './components/adapters/FacebookButton';

// ── Factories ──
export {default as FieldFactory, createField, validateFieldValue} from './components/factories/FieldFactory';
export type {FieldConfig} from './components/factories/FieldFactory';

// ── Utilities ──
export {default as buildThemeConfigFromFlowMeta} from './utils/v2/buildThemeConfigFromFlowMeta';
export {default as getAuthComponentHeadings} from './utils/v2/getAuthComponentHeadings';
export type {HeadingExtractionResult, AuthComponentHeadingsResult} from './utils/v2/getAuthComponentHeadings';

// ── Re-exports from @thunderid/browser ──
export {
  FieldType,
  type AllOrganizationsApiResponse,
  type Config,
  type EmbeddedFlowExecuteRequestPayload,
  type EmbeddedFlowExecuteResponse,
  type EmbeddedSignInFlowHandleRequestPayload,
  type HttpRequestConfig,
  type HttpResponse,
  type IdToken,
  type Organization as IOrganization,
  type SignInOptions,
  type SignOutOptions,
  type SignUpOptions,
  type TokenExchangeRequestConfig,
  type TokenResponse,
  type User as IUser,
} from '@thunderid/browser';

// ── Phase 4 — Utilities ──
export {handleWebAuthnAuthentication} from './utils/handleWebAuthnAuthentication';
export {hasAuthParamsInUrl} from './utils/hasAuthParamsInUrl';
export {navigate} from './utils/navigate';
export {http} from './utils/http';
export {initiateOAuthRedirect} from './utils/oauth';

// ── Phase 4 — Router Helpers ──
export {createThunderIDGuard} from './router/guard';
export type {GuardOptions, ThunderIDNavigationGuard, NavigationGuardReturn} from './router/guard';
export {createCallbackRoute} from './router/callbackRoute';
export type {CallbackRouteOptions, ThunderIDRouteRecord} from './router/callbackRoute';

// ── Phase 4 — Theme Utilities ──
export {getActiveTheme} from './theme/getActiveTheme';
export {detectThemeMode, createClassObserver, createMediaQueryListener} from './theme/themeDetection';
export type {BrowserThemeDetection} from './theme/themeDetection';

// ── Phase 4 — Re-exports from @thunderid/browser (V2 embedded flow models) ──
export {
  ThunderIDRuntimeError,
  EmbeddedFlowComponentTypeV2 as EmbeddedFlowComponentType,
  EmbeddedFlowActionVariantV2 as EmbeddedFlowActionVariant,
  EmbeddedFlowTextVariantV2 as EmbeddedFlowTextVariant,
  EmbeddedFlowEventTypeV2 as EmbeddedFlowEventType,
  type EmbeddedFlowComponentV2 as EmbeddedFlowComponent,
  type EmbeddedFlowResponseDataV2 as EmbeddedFlowResponseData,
  type EmbeddedFlowExecuteRequestConfigV2 as EmbeddedFlowExecuteRequestConfig,
  EmbeddedSignInFlowStatusV2 as EmbeddedSignInFlowStatus,
  EmbeddedSignInFlowTypeV2 as EmbeddedSignInFlowType,
  type ExtendedEmbeddedSignInFlowResponseV2 as ExtendedEmbeddedSignInFlowResponse,
  type EmbeddedSignInFlowResponseV2 as EmbeddedSignInFlowResponse,
  type EmbeddedSignInFlowCompleteResponseV2 as EmbeddedSignInFlowCompleteResponse,
  type EmbeddedSignInFlowInitiateRequestV2 as EmbeddedSignInFlowInitiateRequest,
  type EmbeddedSignInFlowRequestV2 as EmbeddedSignInFlowRequest,
  type EmbeddedSignUpFlowStatusV2 as EmbeddedSignUpFlowStatus,
  type EmbeddedSignUpFlowTypeV2 as EmbeddedSignUpFlowType,
  type ExtendedEmbeddedSignUpFlowResponseV2 as ExtendedEmbeddedSignUpFlowResponse,
  type EmbeddedSignUpFlowResponseV2 as EmbeddedSignUpFlowResponse,
  type EmbeddedSignUpFlowCompleteResponseV2 as EmbeddedSignUpFlowCompleteResponse,
  type EmbeddedSignUpFlowInitiateRequestV2 as EmbeddedSignUpFlowInitiateRequest,
  type EmbeddedSignUpFlowRequestV2 as EmbeddedSignUpFlowRequest,
  type EmbeddedSignUpFlowErrorResponseV2 as EmbeddedSignUpFlowErrorResponse,
  type ComponentRenderContext,
  type ComponentsExtensions,
  type ComponentRenderer,
} from '@thunderid/browser';
