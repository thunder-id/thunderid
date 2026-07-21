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

import type {McpDiscoveryEndpointRow, McpDiscoveryEndpoints} from '../models/mcp-client';

/**
 * Derives the labeled, copyable discovery endpoint rows shown by the mcp-client template's
 * Connect completion screen (`McpConnectComplete`): the issuer, OAuth Authorization Server
 * metadata URL, OIDC discovery URL, authorization endpoint, and token endpoint. Rows whose
 * value is missing/undefined are omitted (graceful degrade, no error).
 *
 * @param wellKnown - The parsed OIDC discovery document, or `null`/`undefined` if not yet loaded
 * @param t - Translation function used to resolve each row's label, with an inline English fallback
 * @returns The endpoint rows with a value present
 *
 * @example
 * ```ts
 * const rows = getMcpDiscoveryEndpointRows(discovery.wellKnown, t);
 * ```
 *
 * @public
 */
export default function getMcpDiscoveryEndpointRows(
  wellKnown: McpDiscoveryEndpoints | null | undefined,
  t: (key: string, fallback: string) => string,
): McpDiscoveryEndpointRow[] {
  const issuer = wellKnown?.issuer;
  const authorizationEndpoint = wellKnown?.authorization_endpoint;
  const tokenEndpoint = wellKnown?.token_endpoint;
  const oauthMetadataUrl = issuer ? `${issuer}/.well-known/oauth-authorization-server` : undefined;
  const oidcDiscoveryUrl = issuer ? `${issuer}/.well-known/openid-configuration` : undefined;

  const allEndpointRows: {key: string; label: string; value?: string}[] = [
    {key: 'issuer', label: t('applications:onboarding.mcp.complete.endpoints.issuer', 'Issuer'), value: issuer},
    {
      key: 'oauthMetadata',
      label: t('applications:onboarding.mcp.complete.endpoints.asMetadata', 'Authorization server metadata'),
      value: oauthMetadataUrl,
    },
    {
      key: 'oidcDiscovery',
      label: t('applications:onboarding.mcp.complete.endpoints.oidcDiscovery', 'OpenID Connect discovery'),
      value: oidcDiscoveryUrl,
    },
    {
      key: 'authorize',
      label: t('applications:onboarding.mcp.complete.endpoints.authorize', 'Authorization endpoint'),
      value: authorizationEndpoint,
    },
    {
      key: 'token',
      label: t('applications:onboarding.mcp.complete.endpoints.token', 'Token endpoint'),
      value: tokenEndpoint,
    },
  ];

  return allEndpointRows.filter((row): row is McpDiscoveryEndpointRow => Boolean(row.value));
}
