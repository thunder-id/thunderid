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

/**
 * Token Configuration
 *
 * Base configuration for OAuth2 tokens including validity period and user attributes.
 * This configuration is shared between access tokens and ID tokens.
 *
 * @public
 * @example
 * ```typescript
 * const accessTokenConfig: TokenConfig = {
 *   validityPeriod: 3600, // 1 hour
 *   userAttributes: ['email', 'username', 'roles']
 * };
 * ```
 */
export interface TokenConfig {
  /**
   * Token validity period in seconds
   * Determines how long the token remains valid after issuance
   * @example 3600 (1 hour)
   */
  validityPeriod: number;

  /**
   * User attributes to include in the token
   * List of user profile attributes that should be included in the token claims
   * @example ['email', 'username', 'given_name', 'family_name']
   */
  userAttributes: string[];
}

/**
 * Assertion Configuration
 *
 * Application-level assertion configuration for non-OAuth flows.
 * This is an alias for TokenConfig with the same properties.
 */
export type AssertionConfig = TokenConfig;

/**
 * Access Token Sub-Configuration
 *
 * Validity period and attribute selection for one access-token subject type.
 *
 * @public
 */
export interface AccessTokenSubConfig {
  /**
   * Token validity period in seconds.
   */
  validityPeriod?: number;

  /**
   * Attributes to include in the access token.
   */
  attributes?: string[];
}

/**
 * Access Token Configuration
 *
 * Access token configuration split by token subject: an end user (userConfig) or the OAuth
 * client itself, issued only via the client_credentials grant (clientConfig).
 *
 * @public
 */
export interface AccessTokenConfig {
  /**
   * Configuration applied when the access token's subject is an end user.
   */
  userConfig?: AccessTokenSubConfig;

  /**
   * Configuration applied when the access token's subject is the OAuth client itself
   * (client_credentials grant).
   */
  clientConfig?: AccessTokenSubConfig;
}
