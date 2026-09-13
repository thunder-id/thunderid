// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Checks that a redirect URI is well formed, tolerating host wildcards. Wildcards in the host are
 * replaced with a placeholder so `new URL()` can parse it; path wildcards (e.g. `/callback/*`) parse
 * natively. The backend enforces the actual wildcard rules (allowed patterns, server config).
 *
 * Empty or whitespace-only input is rejected. Used by both the redirect and post-logout redirect
 * URI fields and the page-level save validation, so a valid wildcard URI never disables Save.
 *
 * @param uri - The redirect URI to check
 * @returns Whether the URI is a parseable, non-empty redirect URI
 *
 * @public
 */
export default function isValidRedirectUriFormat(uri: string): boolean {
  if (!uri.trim()) return false;

  try {
    const schemeEnd = uri.indexOf('://');
    let uriForValidation = uri;
    if (schemeEnd !== -1) {
      const pathStart = uri.indexOf('/', schemeEnd + 3);
      const hostPart = pathStart !== -1 ? uri.slice(schemeEnd + 3, pathStart) : uri.slice(schemeEnd + 3);
      if (hostPart.includes('*')) {
        const sanitizedHost = hostPart.replace(/\*/g, 'wildcard-placeholder');
        uriForValidation = uri.slice(0, schemeEnd + 3) + sanitizedHost + (pathStart !== -1 ? uri.slice(pathStart) : '');
      }
    }
    // eslint-disable-next-line no-new
    new URL(uriForValidation);
    return true;
  } catch {
    return false;
  }
}

/**
 * Allows insecure HTTP redirect URIs only when they use an exact loopback hostname.
 * HTTPS and custom-scheme redirect URIs are unaffected.
 */
export function isValidRedirectUriTransport(uri: string): boolean {
  try {
    const trimmedUri = uri.trim();
    const parsedUri = new URL(trimmedUri);
    if (parsedUri.protocol !== 'http:') return true;

    const hostname = /^http:\/\/(?:[^@/?#]+@)?(\[[^\]]+\]|[^:/?#]+)(?::\d+)?(?:[/?#]|$)/i
      .exec(trimmedUri)?.[1]
      ?.toLowerCase();
    return hostname !== undefined && ['localhost', '127.0.0.1', '[::1]'].includes(hostname);
  } catch {
    return false;
  }
}
