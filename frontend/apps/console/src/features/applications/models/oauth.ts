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

import type {TokenConfig} from './token';

/**
 * OAuth2 Grant Type
 *
 * Supported OAuth2 grant types in the platform.
 *
 * @public
 */
export type OAuth2GrantType =
  | 'authorization_code'
  | 'refresh_token'
  | 'client_credentials'
  | 'password'
  | 'implicit'
  | 'urn:openid:params:grant-type:ciba';

/**
 * OAuth2 Grant Type Constants
 *
 * Constant values for OAuth2 grant types.
 * Use these constants instead of hardcoding strings.
 *
 * @public
 * @example
 * ```typescript
 * const config = {
 *   grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN]
 * };
 * ```
 */
export const OAuth2GrantTypes = {
  /** Authorization Code Flow - Most secure flow for web applications */
  AUTHORIZATION_CODE: 'authorization_code',
  /** Refresh Token - Used to obtain new access tokens */
  REFRESH_TOKEN: 'refresh_token',
  /** Client Credentials - For machine-to-machine authentication */
  CLIENT_CREDENTIALS: 'client_credentials',
  /** Resource Owner Password Credentials - Direct username/password exchange */
  PASSWORD: 'password',
  /** Implicit Flow - Deprecated, for legacy client-side apps */
  IMPLICIT: 'implicit',
  /** Client-Initiated Backchannel Authentication (CIBA) - Decoupled authentication flow */
  CIBA: 'urn:openid:params:grant-type:ciba',
} as const;

/**
 * OAuth2 Response Type
 *
 * Supported OAuth2 response types for authorization requests.
 *
 * @public
 */
export type OAuth2ResponseType = 'code' | 'token' | 'id_token' | 'code token' | 'code id_token' | 'token id_token';

/**
 * OAuth2 Response Type Constants
 *
 * Constant values for OAuth2 response types.
 *
 * @public
 */
export const OAuth2ResponseTypes = {
  /** Authorization code response */
  CODE: 'code',
  /** Access token response (implicit flow) */
  TOKEN: 'token',
  /** ID token response (implicit flow) */
  ID_TOKEN: 'id_token',
  /** Code and token response */
  CODE_TOKEN: 'code token',
  /** Code and ID token response */
  CODE_ID_TOKEN: 'code id_token',
  /** Token and ID token response */
  TOKEN_ID_TOKEN: 'token id_token',
} as const;

/**
 * Token Endpoint Authentication Method
 *
 * Methods for authenticating the client at the token endpoint.
 *
 * @public
 */
export type TokenEndpointAuthMethod =
  | 'client_secret_basic'
  | 'client_secret_post'
  | 'client_secret_jwt'
  | 'private_key_jwt'
  | 'none';

/**
 * Token Endpoint Authentication Method Constants
 *
 * @public
 */
export const TokenEndpointAuthMethods = {
  /** HTTP Basic Authentication with client credentials */
  CLIENT_SECRET_BASIC: 'client_secret_basic',
  /** Client credentials in POST body */
  CLIENT_SECRET_POST: 'client_secret_post',
  /** JWT signed with client secret */
  CLIENT_SECRET_JWT: 'client_secret_jwt',
  /** JWT signed with private key */
  PRIVATE_KEY_JWT: 'private_key_jwt',
  /** No authentication (public clients) */
  NONE: 'none',
} as const;

/**
 * Scope Claims Mapping
 *
 * Maps OAuth2 scopes to the claims (user attributes) they should include.
 * Used in ID tokens to control which user information is exposed for each scope.
 *
 * @public
 * @remarks
 * Standard OIDC scopes include:
 * - `profile`: Name, family name, given name, middle name, nickname, preferred username, picture, website, gender, birthdate, zoneinfo, locale, updatedAt
 * - `email`: Email address and email_verified flag
 * - `phone`: Phone number and phone_number_verified flag
 * - `address`: Formatted address, street address, locality, region, postal code, country
 * - `group`: Group memberships
 *
 * @example
 * ```typescript
 * const scopeClaims: ScopeClaims = {
 *   profile: ['given_name', 'family_name', 'picture'],
 *   email: ['email', 'email_verified'],
 *   phone: ['phone_number'],
 *   group: ['groups']
 * };
 * ```
 */
export interface ScopeClaims {
  /**
   * Claims included when 'profile' scope is requested
   * Typically includes name, picture, and other profile information
   */
  profile?: string[];

  /**
   * Claims included when 'email' scope is requested
   * Typically includes email address and verification status
   */
  email?: string[];

  /**
   * Claims included when 'phone' scope is requested
   * Typically includes phone number and verification status
   */
  phone?: string[];

  /**
   * Claims included when 'group' scope is requested
   * Typically includes user's group memberships
   */
  group?: string[];

  /**
   * Custom scope mappings
   * Allows defining claims for custom scopes beyond the standard OIDC scopes
   */
  [key: string]: string[] | undefined;
}

/**
 * ID Token Configuration
 *
 * Configuration specific to OpenID Connect ID tokens.
 *
 * @public
 */
export type IDTokenConfig = TokenConfig;

/**
 * Refresh Token Configuration
 *
 * Configuration specific to OAuth2 refresh tokens.
 *
 * @public
 */
export interface RefreshTokenConfig {
  /**
   * Token validity period in seconds
   * Determines how long the refresh token remains valid after issuance
   * @example 86400 (24 hours)
   */
  validityPeriod: number;
}

/**
 * OAuth2 Token Settings
 *
 * Complete token configuration for access tokens, ID tokens, and refresh tokens.
 *
 * @public
 * @remarks
 * This configuration is used in the OAuth2 inbound authentication settings
 * to define how tokens should be generated and what information they contain.
 *
 * @example
 * ```typescript
 * const tokenSettings: OAuth2Token = {
 *   accessToken: {
 *     validityPeriod: 3600,
 *     userAttributes: ['email', 'username']
 *   },
 *   idToken: {
 *     validityPeriod: 3600,
 *     userAttributes: ['sub', 'email', 'name'],
 *     scopeClaims: {
 *       profile: ['name', 'picture'],
 *       email: ['email', 'email_verified']
 *     }
 *   },
 *   refreshToken: {
 *     validityPeriod: 86400
 *   }
 * };
 * ```
 */
export interface OAuth2Token {
  /**
   * Access token configuration
   * Defines the validity period and included user attributes for access tokens
   */
  accessToken: TokenConfig;

  /**
   * ID token configuration
   * Defines the validity period, user attributes, and scope-to-claims mapping for ID tokens
   */
  idToken: IDTokenConfig;

  /**
   * Refresh token configuration
   * Defines the validity period for refresh tokens
   */
  refreshToken?: RefreshTokenConfig;
}

/**
 * OAuth2 Configuration
 *
 * Complete OAuth2/OIDC configuration for an application's inbound authentication.
 * This includes client credentials, allowed OAuth2 flows, redirect URIs,
 * security settings (PKCE, public client), scopes, and token configuration.
 *
 * @public
 * @remarks
 * This configuration is used when creating or updating an application
 * to define how OAuth2/OIDC authentication should work for that application.
 *
 * Key security considerations:
 * - Use `pkceRequired: true` for mobile and SPA applications
 * - Set `publicClient: true` only for applications that cannot securely store credentials
 * - Validate all redirectUris to prevent open redirect vulnerabilities
 * - Use authorization_code grant with PKCE for the most secure flow
 *
 * @example
 * ```typescript
 * // Secure web application configuration
 * const webAppConfig: OAuth2Config = {
 *   clientId: 'my-web-app',
 *   clientSecret: 'super-secret-value',
 *   redirectUris: ['https://myapp.com/callback'],
 *   grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN],
 *   responseTypes: [OAuth2ResponseTypes.CODE],
 *   tokenEndpointAuthMethod: TokenEndpointAuthMethods.CLIENT_SECRET_BASIC,
 *   pkceRequired: false,
 *   publicClient: false,
 *   scopes: ['openid', 'profile', 'email'],
 *   token: {
 *     access_token: { validity_period: 3600, user_attributes: ['email', 'username'] },
 *     id_token: { validity_period: 3600, user_attributes: ['sub', 'email', 'name'] }
 *   },
 *   scope_claims: {
 *     profile: ['name', 'picture'],
 *     email: ['email', 'email_verified']
 *   }
 * };
 * ```
 */
export interface OAuth2Config {
  /**
   * OAuth2 client identifier
   * Unique identifier for the application
   * Generated by the server if not provided during creation
   * @example 'my-web-app-client-id'
   */
  clientId?: string;

  /**
   * OAuth2 client secret
   * Secret credential for authenticating the client
   * Required for confidential clients, not used for public clients
   * Should be securely stored and never exposed to end users
   * @example 'super-secret-value'
   */
  clientSecret?: string;

  /**
   * Allowed redirect URIs
   * List of valid URIs where the authorization server can redirect the user after authentication
   * All URIs must be pre-registered to prevent open redirect attacks
   * @example ['https://myapp.com/callback', 'https://myapp.com/oauth/callback']
   */
  redirectUris?: string[];

  /**
   * Allowed OAuth2 grant types
   * Defines which OAuth2 flows the application can use
   * @example [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN]
   */
  grantTypes: string[];

  /**
   * Allowed OAuth2 response types
   * Defines what the authorization endpoint should return
   * @example [OAuth2ResponseTypes.CODE]
   */
  responseTypes: string[];

  /**
   * Token endpoint authentication method
   * Defines how the client authenticates at the token endpoint
   * @defaultValue 'client_secret_basic'
   * @example TokenEndpointAuthMethods.CLIENT_SECRET_BASIC
   */
  tokenEndpointAuthMethod?: string;

  /**
   * Whether PKCE (Proof Key for Code Exchange) is required
   * Should be true for mobile and single-page applications
   * Provides additional security against authorization code interception attacks
   * @defaultValue false
   * @see https://oauth.net/2/pkce/
   */
  pkceRequired?: boolean;

  /**
   * Whether this is a public client
   * Public clients cannot securely store credentials (e.g., SPAs, mobile apps)
   * If true, clientSecret should not be used
   * @defaultValue false
   */
  publicClient?: boolean;

  /**
   * OAuth2/OIDC scopes
   * List of scopes the application can request
   * Standard OIDC scopes: openid, profile, email, phone, address
   * @example ['openid', 'profile', 'email']
   */
  scopes?: string[];

  /**
   * Token configuration
   * Defines how access tokens and ID tokens are generated
   */
  token?: OAuth2Token & Partial<TokenConfig>;

  /**
   * User Info configuration
   * Defines which attributes are returned in the user info response
   */
  userInfo?: UserInfoConfig;

  /**
   * Scope to user-attribute mappings
   * Defines which user attributes are included in tokens when each scope is requested.
   * Stored at the top level of the OAuth2 config (not inside token.id_token).
   * @example { profile: ['name', 'given_name'], email: ['email', 'email_verified'] }
   */
  scopeClaims?: ScopeClaims;

  /**
   * OAuth client certificate (JWKS or JWKS URI).
   * Required when tokenEndpointAuthMethod is 'private_key_jwt'.
   * null means no certificate is configured.
   */
  certificate?: {type: string; value?: string} | null;
}

/**
 * User Info Configuration
 *
 * Configuration specific to the User Info endpoint response.
 * Allows defining which user attributes should be returned in the user info response.
 *
 * @public
 */
export interface UserInfoConfig {
  /**
   * List of user attributes to include in the user info response
   */
  userAttributes: string[];
}
