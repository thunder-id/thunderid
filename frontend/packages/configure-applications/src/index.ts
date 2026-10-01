// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// API Hooks
export {default as useGetApplication} from './api/useGetApplication';
export {default as useGetApplications} from './api/useGetApplications';
export type {UseGetApplicationsParams} from './api/useGetApplications';

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

// Constants
export {default as ApplicationQueryKeys} from './constants/application-query-keys';

// Pages
export {default as ApplicationCreatePage} from './pages/ApplicationCreatePage';
export {default as ApplicationEditPage} from './pages/ApplicationEditPage';
export {default as ApplicationsListPage} from './pages/ApplicationsListPage';
export {default as ApplicationTemplateSelectPage} from './pages/ApplicationTemplateSelectPage';

// Contexts
export {default as ApplicationCreateProvider} from './contexts/ApplicationCreate/ApplicationCreateProvider';

// Routes
export {default as useApplicationRoutes, defaultApplicationRoutePaths} from './hooks/useApplicationRoutes';
export type {ApplicationRoutePaths} from './hooks/useApplicationRoutes';

// Components shared with other configure-* packages
export {default as CopyableField} from './components/common/CopyableField';
export type {CopyableFieldProps} from './components/common/CopyableField';
export {default as SettingsLockNotice} from './components/common/SettingsLockNotice';
export {default as TokenAudienceSelector} from './components/common/TokenAudienceSelector';
export type {TokenAudienceOption} from './components/common/TokenAudienceSelector';
export {default as AuthenticationFlowSection} from './components/edit-application/flows-settings/AuthenticationFlowSection';
export {default as RegistrationFlowSection} from './components/edit-application/flows-settings/RegistrationFlowSection';
export {default as ClientAccessTokenSection} from './components/edit-application/token-settings/ClientAccessTokenSection';
export {default as EditTokenSettings} from './components/edit-application/token-settings/EditTokenSettings';

// Constants and utilities shared with other configure-* packages
export {default as CertificateTypes} from './constants/certificate-types';
export {default as TokenConstants} from './constants/token-constants';
export {getGrantTypeLabel} from './utils/getGrantTypeLabel';
export {applyGrantTypesChange, applyTokenEndpointAuthMethodChange, deriveOAuth2Flags} from './utils/oauth2Rules';
export type {OAuth2Flags} from './utils/oauth2Rules';

// Application templates
export {default as TechnologyBasedApplicationTemplateMetadata} from './config/TechnologyBasedApplicationTemplateMetadata';
export {TechnologyApplicationTemplate} from './models/application-templates';
export type {ApplicationTemplateMetadata} from './models/application-templates';
