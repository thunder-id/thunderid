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

import type {OAuth2Config} from './oauth';

/**
 * Inbound Authentication Protocol Type
 *
 * Supported inbound authentication protocols in the platform.
 * Currently supports OAuth2/OIDC for application authentication.
 *
 * @public
 */
export type InboundAuthType = 'oauth2';

/**
 * Inbound Authentication Type Constants
 *
 * Constant values for inbound authentication protocol types.
 * Use these constants instead of hardcoding strings.
 *
 * @public
 * @example
 * ```typescript
 * const authConfig = {
 *   type: InboundAuthTypes.OAUTH2
 * };
 * ```
 */
export const InboundAuthTypes = {
  /** OAuth 2.0 / OpenID Connect authentication */
  OAUTH2: 'oauth2',
} as const;

/**
 * Inbound Authentication Configuration
 *
 * Defines the inbound authentication protocol and its configuration for an application.
 * Inbound authentication controls how external clients authenticate to access the application's resources.
 *
 * @public
 * @remarks
 * Currently, product supports OAuth2/OIDC as the primary inbound authentication protocol.
 * This configuration is used when creating or updating an application to define:
 * - The authentication protocol type
 * - Protocol-specific configuration (OAuth2 settings)
 *
 * In the future, additional authentication protocols may be supported (e.g., SAML, WS-Federation).
 *
 * @example
 * ```typescript
 * // OAuth2 inbound authentication for a web application
 * const inboundAuth: InboundAuthConfig = {
 *   type: InboundAuthTypes.OAUTH2,
 *   config: {
 *     clientId: 'my-web-app',
 *     clientSecret: 'super-secret',
 *     redirectUris: ['https://myapp.com/callback'],
 *     grantTypes: ['authorization_code', 'refresh_token'],
 *     responseTypes: ['code'],
 *     scopes: ['openid', 'profile', 'email'],
 *     token: {
 *       accessToken: {
 *         userConfig: {
 *           validityPeriod: 3600,
 *           attributes: ['email', 'username']
 *         }
 *       },
 *       idToken: {
 *         validityPeriod: 3600,
 *         userAttributes: ['sub', 'email', 'name'],
 *         scopeClaims: {
 *           profile: ['name', 'picture'],
 *           email: ['email', 'email_verified']
 *         }
 *       }
 *     }
 *   }
 * };
 * ```
 *
 * @example
 * ```typescript
 * // OAuth2 inbound authentication for a SPA with PKCE
 * const spaInboundAuth: InboundAuthConfig = {
 *   type: InboundAuthTypes.OAUTH2,
 *   config: {
 *     redirectUris: ['http://localhost:3000/callback'],
 *     grantTypes: ['authorization_code', 'refresh_token'],
 *     responseTypes: ['code'],
 *     pkceRequired: true,
 *     publicClient: true,
 *     scopes: ['openid', 'profile', 'email'],
 *     token: {
 *       accessToken: {
 *         userConfig: {
 *           validityPeriod: 3600,
 *           attributes: ['email']
 *         }
 *       },
 *       idToken: {
 *         validityPeriod: 3600,
 *         userAttributes: ['sub', 'email'],
 *         scopeClaims: {
 *           email: ['email', 'email_verified']
 *         }
 *       }
 *     }
 *   }
 * };
 * ```
 */
export interface InboundAuthConfig {
  /**
   * The authentication protocol type
   * Currently only 'oauth2' is supported
   * @example InboundAuthTypes.OAUTH2
   */
  type: string;

  /**
   * Protocol-specific configuration
   * For OAuth2/OIDC, this contains client credentials, allowed flows,
   * redirect URIs, scopes, and token settings
   * @see OAuth2Config
   */
  config: OAuth2Config;
}
