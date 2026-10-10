// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Result of validating a back-channel logout URI.
 *
 * @public
 */
export interface BackchannelLogoutUriValidationResult {
  /**
   * Whether the URI can be registered.
   */
  valid: boolean;

  /**
   * The i18n key describing why the URI is invalid. Only set when `valid` is `false`.
   */
  errorKey?: string;

  /**
   * The English fallback for `errorKey`. Only set when `valid` is `false`.
   */
  errorDefault?: string;
}

// eslint-disable-next-line no-control-regex
const CONTROL_CHARACTER = /[\u0000-\u001f\u007f]/;
const MALFORMED_PERCENT_ESCAPE = /%(?![0-9a-fA-F]{2})/;
const HAS_AUTHORITY = /^[a-z][a-z0-9+.-]*:\/\/[^/?]/i;

const INVALID: BackchannelLogoutUriValidationResult = {
  valid: false,
  errorKey: 'applications:edit.general.backchannelLogoutUri.error.invalid',
  errorDefault: 'Enter an absolute http or https URL with a host, and no user info, fragment, or wildcard.',
};

const REQUIRES_HTTPS: BackchannelLogoutUriValidationResult = {
  valid: false,
  errorKey: 'applications:edit.general.backchannelLogoutUri.error.requiresHttps',
  errorDefault: 'A public client must use an https URL.',
};

/**
 * Validates a back-channel logout URI with the rules the server applies on save: an empty value is
 * allowed and means the client is not notified; otherwise the URI must be an absolute `http` or
 * `https` URL with a host, without user info, a fragment (not even an empty one), or a `*`, and a
 * public client must use `https`. Whether private and loopback hosts are refused depends on the
 * server's configuration, so that check is left to the server.
 *
 * @param uri - The back-channel logout URI to validate
 * @param publicClient - Whether the client is public
 * @returns The validation result, with an `errorKey` and its fallback set when the URI is invalid
 *
 * @example
 * ```ts
 * validateBackchannelLogoutUri('https://rp.example.com/bcl', true); // { valid: true }
 * validateBackchannelLogoutUri('http://rp.example.com/bcl', true); // { valid: false, errorKey: '...requiresHttps' }
 * ```
 *
 * @public
 */
export default function validateBackchannelLogoutUri(
  uri: string | undefined,
  publicClient: boolean,
): BackchannelLogoutUriValidationResult {
  const trimmedUri = (uri ?? '').trim();
  if (!trimmedUri) {
    return {valid: true};
  }
  if (trimmedUri.includes('*') || trimmedUri.includes('#')) {
    return INVALID;
  }
  // The browser's URL parser drops tab and newline characters and accepts a malformed percent escape,
  // both of which the server rejects, so refuse them before parsing.
  if (CONTROL_CHARACTER.test(trimmedUri) || MALFORMED_PERCENT_ESCAPE.test(trimmedUri)) {
    return INVALID;
  }
  // The browser's URL parser normalizes what the server rejects: `https:///path` becomes
  // `https://path/`, an empty `@` before the host disappears, and `\` turns into `/`. So check the authority as
  // written: it must be present, without `@` (user info, even empty), and no `\` may appear at all.
  if (!HAS_AUTHORITY.test(trimmedUri) || trimmedUri.includes('\\')) {
    return INVALID;
  }
  const authority = trimmedUri.slice(trimmedUri.indexOf('://') + 3).split(/[/?]/)[0];
  if (authority.includes('@')) {
    return INVALID;
  }

  let parsedUri: URL;
  try {
    parsedUri = new URL(trimmedUri);
  } catch {
    return INVALID;
  }
  const isHttp = parsedUri.protocol === 'http:';
  if ((!isHttp && parsedUri.protocol !== 'https:') || !parsedUri.hostname) {
    return INVALID;
  }
  if (isHttp && publicClient) {
    return REQUIRES_HTTPS;
  }

  return {valid: true};
}
