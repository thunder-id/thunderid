// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The redirect URIs a new application is created with. Entered ones replace the template's, keeping a
 * callback URL entered on the details step; with none entered, the callback URL or the template's are used.
 */
export default function resolveRedirectUris(
  templateUris: string[],
  callbackUrl: string,
  enteredUris: string[],
): string[] {
  const entered = enteredUris.map((uri) => uri.trim()).filter(Boolean);
  if (entered.length === 0) {
    return callbackUrl ? [callbackUrl] : templateUris;
  }
  return Array.from(new Set([...(callbackUrl ? [callbackUrl] : []), ...entered]));
}
