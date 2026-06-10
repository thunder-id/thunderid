/**
 * Copyright (c) 2020-2026, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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

export {IsomorphicCrypto} from './IsomorphicCrypto';

export {default as executeEmbeddedSignInFlow} from './api/executeEmbeddedSignInFlow';
export {default as executeEmbeddedSignUpFlow} from './api/executeEmbeddedSignUpFlow';
export {default as executeEmbeddedRecoveryFlow} from './api/executeEmbeddedRecoveryFlow';
export {default as executeEmbeddedUserOnboardingFlow} from './api/executeEmbeddedUserOnboardingFlow';
export type {EmbeddedUserOnboardingFlowResponse} from './api/executeEmbeddedUserOnboardingFlow';
export {default as getFlowMeta} from './api/getFlowMeta';
export {default as getOrganizationUnitChildren} from './api/getOrganizationUnitChildren';
export {default as getUserInfo} from './api/getUserInfo';
export {default as getScim2Me} from './api/getScim2Me';
export type {GetScim2MeConfig} from './api/getScim2Me';
export {default as getSchemas} from './api/getSchemas';
export type {GetSchemasConfig} from './api/getSchemas';
export {default as getAllOrganizations} from './api/getAllOrganizations';
export type {GetAllOrganizationsConfig} from './api/getAllOrganizations';
export {default as createOrganization} from './api/createOrganization';
export type {CreateOrganizationPayload, CreateOrganizationConfig} from './api/createOrganization';
export {default as getMeOrganizations} from './api/getMeOrganizations';
export type {GetMeOrganizationsConfig} from './api/getMeOrganizations';
export {default as getOrganization} from './api/getOrganization';
export type {OrganizationDetails, GetOrganizationConfig} from './api/getOrganization';
export {default as updateOrganization, createPatchOperations} from './api/updateOrganization';
export type {UpdateOrganizationConfig} from './api/updateOrganization';
export {default as updateMeProfile} from './api/updateMeProfile';
export type {UpdateMeProfileConfig} from './api/updateMeProfile';
export {default as getBrandingPreference} from './api/getBrandingPreference';
export type {GetBrandingPreferenceConfig} from './api/getBrandingPreference';

export {default as ApplicationNativeAuthenticationConstants} from './constants/ApplicationNativeAuthenticationConstants';
export {default as TokenConstants} from './constants/TokenConstants';
export {default as OIDCRequestConstants} from './constants/OIDCRequestConstants';
export {default as VendorConstants} from './constants/VendorConstants';

export {default as ThunderIDError} from './errors/ThunderIDError';
export {default as ThunderIDAPIError} from './errors/ThunderIDAPIError';
export {default as ThunderIDRuntimeError} from './errors/ThunderIDRuntimeError';
export {ThunderIDAuthException} from './errors/exception';

export type {AllOrganizationsApiResponse} from './models/organization';
export {Platform} from './models/platforms';
export {
  EmbeddedFlowType,
  EmbeddedFlowResponseType,
  EmbeddedFlowComponentType,
  EmbeddedFlowActionVariant,
  EmbeddedFlowTextVariant,
  EmbeddedFlowEventType,
} from './models/embedded-flow';
export type {
  EmbeddedFlowComponent,
  EmbeddedFlowResponseData,
  EmbeddedFlowExecuteRequestConfig,
  FlowExecutionError,
  ConsentAttributeElement,
  ConsentPurposeDecision,
  ConsentDecisions,
  ConsentPurposeData,
  ConsentPromptData,
  I18nMessage,
} from './models/embedded-flow';
export {
  EmbeddedSignInFlowStatus,
  EmbeddedSignInFlowType,
} from './models/embedded-signin-flow';
export type {
  ExtendedEmbeddedSignInFlowResponse,
  EmbeddedSignInFlowResponse,
  EmbeddedSignInFlowCompleteResponse,
  EmbeddedSignInFlowInitiateRequest,
  EmbeddedSignInFlowRequest,
} from './models/embedded-signin-flow';
export {
  EmbeddedSignUpFlowStatus,
  EmbeddedSignUpFlowType,
} from './models/embedded-signup-flow';
export type {
  ExtendedEmbeddedSignUpFlowResponse,
  EmbeddedSignUpFlowResponse,
  EmbeddedSignUpFlowCompleteResponse,
  EmbeddedSignUpFlowInitiateRequest,
  EmbeddedSignUpFlowRequest,
  EmbeddedSignUpFlowErrorResponse,
} from './models/embedded-signup-flow';
export {
  EmbeddedRecoveryFlowStatus,
  EmbeddedRecoveryFlowType,
} from './models/embedded-recovery-flow';
export type {
  EmbeddedRecoveryFlowResponse,
  EmbeddedRecoveryFlowInitiateRequest,
  EmbeddedRecoveryFlowRequest,
  EmbeddedRecoveryFlowErrorResponse,
} from './models/embedded-recovery-flow';
export type {
  OrganizationUnit,
  OrganizationUnitListResponse,
  GetOrganizationUnitChildrenConfig,
} from './models/organization-unit';
export {FlowMetaType} from './models/flow-meta';
export type {
  ApplicationMetadata,
  OUMetadata,
  DesignMetadata,
  I18nMetadata,
  FlowMetadataResponse,
  GetFlowMetaRequestConfig,
  FlowMetaTheme,
  FlowMetaThemeColorSet,
  FlowMetaThemeBackground,
  FlowMetaThemeTextColors,
  FlowMetaThemeColors,
  FlowMetaThemeColorScheme,
  FlowMetaThemeShape,
  FlowMetaThemeTypography,
} from './models/flow-meta';
export {FlowMode} from './models/flow';
export type {ThunderIDClient} from './models/client';
export type {
  AuthClientConfig,
  StrictAuthClientConfig,
  DefaultAuthClientConfig,
  WellKnownAuthClientConfig,
  BaseURLAuthClientConfig,
  ExplicitAuthClientConfig,
  BaseConfig,
  Config,
  Preferences,
  ThemePreferences,
  I18nPreferences,
  I18nStorageStrategy,
  WithPreferences,
  Extensions,
  WithExtensions,
  SignInOptions,
  SignOutOptions,
  SignUpOptions,
} from './models/config';
export type {TokenEndpointAuthMethod} from './models/token-endpoint-auth';
export type {ComponentRenderContext, ComponentRenderer, ComponentsExtensions} from './models/extensions/components';
export type {TokenResponse, IdToken, TokenExchangeRequestConfig} from './models/token';
export type {AgentConfig} from './models/agent';
export type {AuthCodeResponse} from './models/auth-code-response';
export type {Crypto, JWKInterface} from './models/crypto';
export type {OAuthResponseMode} from './models/oauth-response';
export type {
  AuthorizeRequestUrlParams,
  KnownExtendedAuthorizeRequestUrlParams,
  ExtendedAuthorizeRequestUrlParams,
} from './models/oauth-request';
export type {OIDCEndpoints} from './models/oidc-endpoints';
export type {OIDCDiscoveryApiResponse} from './models/oidc-discovery';
export type {Storage, TemporaryStore} from './models/store';
export type {User, UserProfile} from './models/user';
export type {SessionData} from './models/session';
export type {Organization} from './models/organization';
export type {TranslationFn} from './models/translation';
export type {ResolveFlowTemplateLiteralsOptions} from './models/vars';
export type {
  BrandingPreference,
  BrandingPreferenceConfig,
  BrandingLayout,
  BrandingTheme,
  ThemeVariant,
  ButtonsConfig,
  ColorsConfig,
  ColorVariants,
  BrandingOrganizationDetails,
  UrlsConfig,
} from './models/branding-preference';
export {WellKnownSchemaIds} from './models/scim2-schema';
export type {Schema, SchemaAttribute, FlattenedSchema} from './models/scim2-schema';
export type {RecursivePartial} from './models/utility-types';
export {FieldType} from './models/field';

export {default as ThunderIDJavaScriptClient} from './ThunderIDJavaScriptClient';

export {default as createTheme, DEFAULT_THEME} from './theme/createTheme';
export type {ThemeColors, ThemeConfig, Theme, ThemeMode, ThemeDetection} from './theme/types';

export {default as AuthenticationHelper} from './utils/AuthenticationHelper';
export {default as arrayBufferToBase64url} from './utils/arrayBufferToBase64url';
export {default as base64urlToArrayBuffer} from './utils/base64urlToArrayBuffer';
export {default as bem} from './utils/bem';
export {default as formatDate} from './utils/formatDate';
export {default as processUsername} from './utils/processUsername';
export {default as deepMerge} from './utils/deepMerge';
export {default as deriveOrganizationHandleFromBaseUrl} from './utils/deriveOrganizationHandleFromBaseUrl';
export {default as extractUserClaimsFromIdToken} from './utils/extractUserClaimsFromIdToken';
export {default as isRecognizedBaseUrlPattern} from './utils/isRecognizedBaseUrlPattern';
export {default as extractPkceStorageKeyFromState} from './utils/extractPkceStorageKeyFromState';
export {default as flattenUserSchema} from './utils/flattenUserSchema';
export {default as generateUserProfile} from './utils/generateUserProfile';
export {default as getLatestStateParam} from './utils/getLatestStateParam';
export {default as generateFlattenedUserProfile} from './utils/generateFlattenedUserProfile';
export {default as getRedirectBasedSignUpUrl} from './utils/getRedirectBasedSignUpUrl';
export {default as identifyPlatform} from './utils/identifyPlatform';
export {default as isEmpty} from './utils/isEmpty';
export {default as isEmojiUri, EMOJI_URI_SCHEME} from './utils/isEmojiUri';
export {default as extractEmojiFromUri} from './utils/extractEmojiFromUri';
export {default as set} from './utils/set';
export {default as get} from './utils/get';
export {default as removeTrailingSlash} from './utils/removeTrailingSlash';
export {default as resolveFieldName} from './utils/resolveFieldName';
export {default as resolveMeta} from './utils/resolveMeta';
export {default as resolveFlowTemplateLiterals} from './utils/resolveFlowTemplateLiterals';
export {default as countryCodeToFlagEmoji} from './utils/countryCodeToFlagEmoji';
export {default as resolveLocaleDisplayName} from './utils/resolveLocaleDisplayName';
export {default as resolveLocaleEmoji} from './utils/resolveLocaleEmoji';
export {default as processOpenIDScopes} from './utils/processOpenIDScopes';
export {default as withVendorCSSClassPrefix} from './utils/withVendorCSSClassPrefix';
export {default as transformBrandingPreferenceToTheme} from './utils/transformBrandingPreferenceToTheme';

export {
  default as logger,
  createLogger,
  createComponentLogger,
  createPackageLogger,
  createPackageComponentLogger,
  configure as configureLogger,
  debug,
  info,
  warn,
  error,
} from './utils/logger';
export type {LogLevel, LoggerConfig} from './utils/logger';

export {default as StorageManager} from './StorageManager';

export {HttpClient} from './HttpClient';
export type {HttpError, HttpRequestConfig, HttpResponse} from './models/http';

export type {I18nBundle, I18nTranslations} from './i18n/models/i18n';
export {default as TranslationBundleConstants} from './i18n/constants/TranslationBundleConstants';
export {default as getDefaultI18nBundles} from './i18n/utils/getDefaultI18nBundles';
export {default as normalizeTranslations} from './i18n/utils/normalizeTranslations';
