// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** The kind of resource a value is loaded as, used to pick the CSP directive to allow it in. */
export type CspResourceType = 'image' | 'stylesheet' | 'font';

/** The origin and guidance message for a value that would need a CSP change. */
export interface CspHint {
  /** The external origin (scheme + host + port) the value loads from. */
  origin: string;

  /** i18n key for the directive-specific guidance message. */
  messageKey: string;
}

// Returns the external origin of an absolute http/https URL, or null for values no CSP directive
// governs: relative paths, data:/blob: URIs, emoji:/avatar: sentinels, or same-origin URLs ('self').
function externalOrigin(value: string): string | null {
  const trimmed = value.trim();

  if (!trimmed) {
    return null;
  }

  let url: URL;
  try {
    url = new URL(trimmed);
  } catch {
    return null;
  }

  if (url.protocol !== 'http:' && url.protocol !== 'https:') {
    return null;
  }

  if (typeof window !== 'undefined' && url.origin === window.location.origin) {
    return null;
  }

  return url.origin;
}

// Resolves the CSP hint for a value, or null when it does not load from an external origin. Google
// Fonts is special-cased: it serves the stylesheet from fonts.googleapis.com and the fonts from
// fonts.gstatic.com, so it needs two directives.
export function resolveCspHint(value: string, resourceType: CspResourceType): CspHint | null {
  const origin = externalOrigin(value);

  if (!origin) {
    return null;
  }

  if (resourceType === 'font') {
    const {host} = new URL(origin);
    if (host === 'fonts.googleapis.com' || host === 'fonts.gstatic.com') {
      return {origin, messageKey: 'common:csp.hint.fontGoogle'};
    }
    return {origin, messageKey: 'common:csp.hint.font'};
  }

  if (resourceType === 'image') {
    return {origin, messageKey: 'common:csp.hint.image'};
  }

  return {origin, messageKey: 'common:csp.hint.stylesheet'};
}
