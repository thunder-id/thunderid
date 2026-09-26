// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API Hooks
export {default as useGetApplication} from './api/useGetApplication';
export {default as useGetApplications} from './api/useGetApplications';
export type {UseGetApplicationsParams} from './api/useGetApplications';
export {default as useCreateApplication} from './api/useCreateApplication';

// Models & Types
export type {Application, ApplicationType, BasicApplication} from './models/application';
export type {ApplicationListResponse} from './models/responses';
export {InboundAuthTypes} from './models/inbound-auth';
export type {InboundAuthConfig, InboundAuthType} from './models/inbound-auth';
export {
  OAuth2GrantTypes,
  OAuth2ResponseTypes,
  REFRESH_TOKEN_ISSUING_GRANTS,
  TokenEndpointAuthMethods,
} from './models/oauth';
export type {
  AndroidAttestationConfig,
  AppleAttestationConfig,
  AttestationConfig,
  IDJAGConfig,
  IDTokenConfig,
  IDTokenResponseType,
  OAuth2Config,
  OAuth2GrantType,
  OAuth2ResponseType,
  OAuth2Token,
  RefreshTokenConfig,
  ScopeClaims,
  TokenEndpointAuthMethod,
  UserInfoConfig,
  UserInfoResponseType,
} from './models/oauth';
export type {AccessTokenConfig, AccessTokenSubConfig, AssertionConfig, TokenConfig} from './models/token';
export type {CreateApplicationRequest} from './models/requests';
export {
  ApplicationCreateFlowConfiguration,
  ApplicationCreateFlowSignInApproach,
  ApplicationCreateFlowStep,
  OrganizationUnitDefaultItem,
} from './models/application-create-flow';
export type {OrganizationUnitDefaultsSelection} from './models/application-create-flow';
export {PlatformApplicationTemplate, TechnologyApplicationTemplate} from './models/application-templates';
export type {
  ApplicationTemplate,
  ApplicationTemplateMetadata,
  IntegrationGuide,
  IntegrationGuides,
  PlaygroundLink,
  QuickstartLink,
  TemplateCategory,
} from './models/application-templates';
export {McpClientTypes} from './models/mcp-client';
export type {McpClientType, McpDiscoveryEndpointRow, McpDiscoveryEndpoints} from './models/mcp-client';

// Constants
export {default as ApplicationQueryKeys} from './constants/application-query-keys';
export {default as ApplicationConstants} from './constants/application-constants';
export {default as TemplateConstants} from './constants/template-constants';
export {default as TokenConstants} from './constants/token-constants';
export {default as CertificateTypes} from './constants/certificate-types';
export {CUSTOM_WALLET_VENDOR, WALLET_VENDORS} from './constants/wallet-vendors';
export type {WalletVendor} from './constants/wallet-vendors';

// Config
export {default as PlatformBasedApplicationTemplateMetadata} from './config/PlatformBasedApplicationTemplateMetadata';
export {default as TechnologyBasedApplicationTemplateMetadata} from './config/TechnologyBasedApplicationTemplateMetadata';
export {default as McpClientTypeMetadataList} from './config/McpClientTypeMetadata';
export type {McpClientTypeMetadata} from './config/McpClientTypeMetadata';

// Utils
export {default as computeOrganizationUnitDefaultAvailability} from './utils/computeOrganizationUnitDefaultAvailability';
export type {OrganizationUnitDefaultAvailabilityFlowFlags} from './utils/computeOrganizationUnitDefaultAvailability';
export {default as getApplicationErrorMessage} from './utils/getApplicationErrorMessage';
export {default as getConfigurationTypeFromTemplate} from './utils/getConfigurationTypeFromTemplate';
export {getGrantTypeLabel} from './utils/getGrantTypeLabel';
export {
  default as getIntegrationGuidesForTemplate,
  getIntegrationGuideForTemplate,
  getIntegrationGuideVariantKey,
} from './utils/getIntegrationGuidesForTemplate';
export {default as getPlaygroundsForTemplate} from './utils/getPlaygroundsForTemplate';
export {default as getQuickstartsForTemplate} from './utils/getQuickstartsForTemplate';
export {default as hasInvalidCorsRows} from './utils/hasInvalidCorsRows';
export {default as isRedirectCapableTemplate} from './utils/isRedirectCapableTemplate';
export {default as mergeCorsOrigins} from './utils/mergeCorsOrigins';
export {default as normalizeTemplateId} from './utils/normalizeTemplateId';
export {
  applyGrantTypesChange,
  applyPublicClientChange,
  applyTokenEndpointAuthMethodChange,
  deriveOAuth2Flags,
  getPkceCaption,
  getPublicClientCaption,
  hasClientAccess,
  hasUserAccess,
  isGrantItemDisabled,
  isOAuthTokenMode,
  USER_ACCESS_GRANTS,
} from './utils/oauth2Rules';
export type {CaptionTuple, OAuth2Flags} from './utils/oauth2Rules';
export {default as resolveApplicationType, isClientCredentialsOnlyGrantSet, isM2MApplication} from './utils/resolveApplicationType';
export {default as resolveCreationFlow} from './utils/resolveCreationFlow';
export {default as resolveTemplateLink} from './utils/resolveTemplateLink';
export {default as validateMcpRedirectUri} from './utils/validateMcpRedirectUri';
export type {McpRedirectUriValidationResult} from './utils/validateMcpRedirectUri';

// Components
export {default as CopyableField} from './components/common/CopyableField';
export type {CopyableFieldProps} from './components/common/CopyableField';
export {default as SettingsLockNotice} from './components/common/SettingsLockNotice';
export {default as TokenAudienceSelector} from './components/common/TokenAudienceSelector';
export type {TokenAudienceOption} from './components/common/TokenAudienceSelector';
export {default as ClientAccessTokenSection} from './components/edit-application/token-settings/ClientAccessTokenSection';
export type {ClientAccessTokenCopy} from './components/edit-application/token-settings/ClientAccessTokenSection';
export {default as EditTokenSettings} from './components/edit-application/token-settings/EditTokenSettings';
export {default as ShowClientSecret} from './components/create-application/ShowClientSecret';

// Pages
export {default as ApplicationsListPage} from './pages/ApplicationsListPage';
export {default as ApplicationEditPage} from './pages/ApplicationEditPage';
export type {
  ApplicationEditPageProps,
  FlowsSettingsRenderProps,
  IntegrationGuidesRenderProps,
} from './pages/ApplicationEditPage';

// Hooks
export {default as useApplicationRoutes, defaultApplicationRoutePaths} from './hooks/useApplicationRoutes';
export type {ApplicationRoutePaths} from './hooks/useApplicationRoutes';
