// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// `new URL` rewrites `https:host`, `https:/host` and `https:///host` into a URL with a host, so the
// literal `scheme://authority` form is checked on the raw value first.
const HTTP_URL_WITH_AUTHORITY = /^https?:\/\/[^/\\]/i;

/**
 * Checks whether a value can be the identifier of an MCP server. The identifier is the canonical server
 * URI that MCP clients send as the RFC 8707 `resource` parameter, so this mirrors the backend rule: an
 * http or https URL with a host and no fragment.
 *
 * @param value - The trimmed identifier to check.
 * @returns `true` when the value is an http or https URL with a host and no fragment.
 *
 * @example
 * isValidMcpServerIdentifier('https://mcp.example.com/mcp'); // true
 * isValidMcpServerIdentifier('localhost:8000/mcp'); // false
 *
 * @public
 */
export function isValidMcpServerIdentifier(value: string): boolean {
  // `new URL` reports an empty hash for a trailing `#` and rewrites `\` to `/`, while the backend keeps
  // or refuses them, so the raw value is checked for both.
  if (!HTTP_URL_WITH_AUTHORITY.test(value) || /[#\\]/.test(value)) {
    return false;
  }

  // `new URL` decodes a percent-encoded host, which the backend refuses. Userinfo may stay encoded.
  const authority = value.slice(value.indexOf('//') + 2).split(/[/?#]/)[0];
  if (authority.slice(authority.lastIndexOf('@') + 1).includes('%')) {
    return false;
  }

  try {
    const url = new URL(value);

    return (url.protocol === 'http:' || url.protocol === 'https:') && url.hostname !== '' && url.hash === '';
  } catch {
    return false;
  }
}
