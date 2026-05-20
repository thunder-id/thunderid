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

/**
 * Constants for OAuth 2.0 and OpenID Connect scopes.
 * These scopes define the level of access that the client application
 * is requesting from the authorization server.
 *
 * @remarks
 * Scopes are space-separated strings that represent different permissions.
 * The 'openid' scope is required for OpenID Connect flows, while other
 * scopes provide access to different resources or user information.
 *
 * @example
 * ```typescript
 * // Requesting OpenID Connect authentication
 * const scope = [ScopeConstants.OPENID];
 *
 * // Requesting profile information
 * const scopes = [ScopeConstants.OPENID, ScopeConstants.PROFILE];
 * ```
 */
const ScopeConstants: {
  INTERNAL_LOGIN: string;
  OPENID: string;
  PROFILE: string;
} = {
  /**
   * The scope for accessing the user's profile information from SCIM.
   * This scope allows the client to retrieve basic user information such as
   * name, email, profile picture, etc.
   */
  INTERNAL_LOGIN: 'internal_login',

  /**
   * The base OpenID Connect scope.
   * Required for all OpenID Connect flows. Indicates that the client
   * is initiating an OpenID Connect authentication request.
   */
  OPENID: 'openid',

  /**
   * The OpenID Connect profile scope.
   * This scope allows the client to access the user's profile information.
   * It includes details such as the user's name, email, and other profile attributes.
   */
  PROFILE: 'profile',
} as const;

export default ScopeConstants;
