// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isValidRedirectUriTransport} from './isValidRedirectUriFormat';

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
    const isLoopbackHttp = parsedUri.protocol === 'http:' && isValidRedirectUriTransport(trimmedUri);
    const isHttps = parsedUri.protocol === 'https:';

    if (isLoopbackHttp || isHttps) {
      return {valid: true};
    }
  } catch {
    // Falls through to the invalid result below.
  }

  return {valid: false, errorKey: 'applications:onboarding.mcp.connection.redirectUris.error.invalid'};
}
