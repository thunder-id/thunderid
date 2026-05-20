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
 * Constants related to OpenID Connect (OIDC) metadata and endpoints.
 * This object contains all the standard OIDC endpoints and storage keys
 * used throughout the application for authentication and authorization.
 *
 * @remarks
 * The constants are organized into two main sections:
 * 1. Endpoints - Contains all OIDC standard endpoint paths
 * 2. Storage - Contains keys used for storing OIDC-related data
 *
 * @example
 * ```typescript
 * // Using an endpoint
 * const wellKnownEndpoint = OIDCDiscoveryConstants.Endpoints.WELL_KNOWN;
 * ```
 */
const OIDCDiscoveryConstants: {
  readonly Endpoints: {
    readonly WELL_KNOWN: string;
  };
} = {
  /**
   * Collection of standard OIDC endpoint paths used for authentication flows.
   * These endpoints are relative paths that should be appended to the base URL
   * of your identity provider.
   */
  Endpoints: {
    /**
     * OpenID Connect discovery document endpoint.
     * Used to fetch provider metadata from the authorization server.
     */
    WELL_KNOWN: '/.well-known/openid-configuration',
  },
} as const;

export default OIDCDiscoveryConstants;
