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
 * The MCP client type values, chosen on the mcp-client template's Client type step.
 *
 * @public
 */
export const McpClientTypes = {
  /** Acts on behalf of a signed-in user (Authorization Code + PKCE). */
  USER_DELEGATED: 'userDelegated',
  /** An autonomous agent/service calling MCP servers with its own identity (Client Credentials). */
  M2M: 'm2m',
} as const;

/**
 * The MCP client type, chosen on the mcp-client template's Name & Type step.
 *
 * - `userDelegated` — acts on behalf of a signed-in user (Authorization Code + PKCE).
 * - `m2m` — an autonomous agent or service calling MCP servers with its own identity
 *   (Client Credentials).
 *
 * @public
 */
export type McpClientType = (typeof McpClientTypes)[keyof typeof McpClientTypes];

/**
 * The subset of the OIDC discovery response used by the mcp-client template's Connect
 * completion screen and Connect edit tab.
 *
 * @public
 */
export interface McpDiscoveryEndpoints {
  /**
   * The OAuth 2.1 authorization server / OIDC issuer identifier.
   *
   * @example 'https://localhost:8090'
   */
  issuer?: string;

  /**
   * The OAuth 2.1 authorization endpoint.
   *
   * @example 'https://localhost:8090/oauth2/authorize'
   */
  authorization_endpoint?: string;

  /**
   * The OAuth 2.1 token endpoint.
   *
   * @example 'https://localhost:8090/oauth2/token'
   */
  token_endpoint?: string;
}

/**
 * A single labeled, copyable discovery endpoint row derived from {@link McpDiscoveryEndpoints}.
 *
 * @public
 */
export interface McpDiscoveryEndpointRow {
  /**
   * Stable identifier for the row, used as the React key and copyable field id suffix.
   *
   * @example 'issuer'
   */
  key: string;

  /**
   * The row's translated label.
   *
   * @example 'Issuer'
   */
  label: string;

  /**
   * The endpoint URL.
   *
   * @example 'https://localhost:8090'
   */
  value: string;
}
