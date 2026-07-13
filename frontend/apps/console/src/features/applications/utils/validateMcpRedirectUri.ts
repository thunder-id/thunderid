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
 * Result of validating an MCP client redirect URI.
 *
 * @public
 */
export interface McpRedirectUriValidationResult {
  /**
   * Whether the URI satisfies the MCP redirect URI rule.
   */
  valid: boolean;

  /**
   * The i18n key describing why the URI is invalid. Only set when `valid` is `false`.
   */
  errorKey?: string;
}

/**
 * Loopback hostnames accepted for the `http:` scheme. `[::1]` is how the WHATWG `URL` parser
 * reports the IPv6 loopback hostname (brackets included).
 */
const LOOPBACK_HOSTNAMES = ['localhost', '127.0.0.1', '[::1]'];

/**
 * Validates a redirect URI against the MCP client redirect URI rule: the URI must be a loopback
 * address (`http://localhost[:port]/...`, `http://127.0.0.1[:port]/...`, or
 * `http://[::1][:port]/...`) or use HTTPS. Any other scheme, including plain `http://` on a
 * non-loopback host, is rejected. Wildcards (`*`) are rejected anywhere in the URI — the
 * backend's create-time validation rejects `*` in ports and (by default) hosts, so a
 * wildcard redirect URI can never be registered.
 *
 * @param uri - The redirect URI to validate
 * @returns The validation result, with an `errorKey` set when the URI is invalid
 *
 * @example
 * ```ts
 * validateMcpRedirectUri('https://agent.example.com/oauth/cb'); // { valid: true }
 * validateMcpRedirectUri('http://example.com/cb'); // { valid: false, errorKey: '...error.invalid' }
 * ```
 *
 * @public
 */
export default function validateMcpRedirectUri(uri: string): McpRedirectUriValidationResult {
  const trimmedUri = uri.trim();

  if (!trimmedUri) {
    return {valid: false, errorKey: 'applications:onboarding.mcp.connection.redirectUris.error.empty'};
  }

  if (trimmedUri.includes('*')) {
    return {valid: false, errorKey: 'applications:onboarding.mcp.connection.redirectUris.error.invalid'};
  }

  try {
    const parsedUri = new URL(trimmedUri);
    const isLoopbackHttp = parsedUri.protocol === 'http:' && LOOPBACK_HOSTNAMES.includes(parsedUri.hostname);
    const isHttps = parsedUri.protocol === 'https:';

    if (isLoopbackHttp || isHttps) {
      return {valid: true};
    }
  } catch {
    // Falls through to the invalid result below.
  }

  return {valid: false, errorKey: 'applications:onboarding.mcp.connection.redirectUris.error.invalid'};
}
