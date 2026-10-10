// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Words a field name abbreviates, written as a reader expects them. */
const WORDS: Record<string, string> = {
  id: 'ID',
  ids: 'IDs',
  uri: 'URI',
  uris: 'URIs',
  url: 'URL',
  urls: 'URLs',
  ou: 'OU',
  oauth: 'OAuth',
  oauth2: 'OAuth2',
  pkce: 'PKCE',
  jwks: 'JWKS',
  jwt: 'JWT',
  mcp: 'MCP',
  css: 'CSS',
  cors: 'CORS',
  tos: 'ToS',
  saml: 'SAML',
  oidc: 'OIDC',
  ttl: 'TTL',
};

/**
 * Names a field for a reader: `redirectUris` becomes "Redirect URIs" and `token_endpoint` becomes
 * "Token endpoint".
 */
export function fieldLabel(field: string): string {
  const words = field
    .replace(/([a-z0-9])([A-Z])/g, '$1 $2')
    .replace(/[_-]+/g, ' ')
    .trim()
    .split(/\s+/)
    .filter(Boolean);
  return words
    .map((word: string, index: number) => {
      const known = WORDS[word.toLowerCase()];
      if (known) return known;
      const lower = word.toLowerCase();
      return index === 0 ? lower.charAt(0).toUpperCase() + lower.slice(1) : lower;
    })
    .join(' ');
}
