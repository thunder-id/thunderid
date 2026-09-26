// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ApplicationType} from '../models/application';
import {OAuth2GrantTypes} from '../models/oauth';
import type {OAuth2Config} from '../models/oauth';
const KNOWN_TYPES: readonly ApplicationType[] = ['browser', 'fullstack', 'mobile', 'm2m', 'mcp', 'custom'];

/**
 * Resolves the canonical application type for behavior decisions.
 *
 * Prefers the explicit `type` set at creation. For legacy applications created before the type
 * attribute existed, or when the stored type is `custom` (no enforced shape), falls back to
 * inferring from the OAuth config so existing behavior is preserved. Note that `browser` and
 * `mobile` are indistinguishable from config shape alone, so the fallback resolves public
 * redirect clients to `browser`.
 *
 * @param type - The application's stored `type`, if any.
 * @param oauth2Config - The application's OAuth2 config, used only for the fallback path.
 * @returns The resolved application type.
 */
export default function resolveApplicationType(
  type: ApplicationType | undefined,
  oauth2Config?: OAuth2Config,
): ApplicationType {
  if (type && type !== 'custom' && KNOWN_TYPES.includes(type)) {
    return type;
  }

  const grantTypes = oauth2Config?.grantTypes ?? [];
  if (isClientCredentialsOnlyGrantSet(grantTypes)) {
    return 'm2m';
  }
  if (oauth2Config?.publicClient) {
    return 'browser';
  }
  if (grantTypes.includes(OAuth2GrantTypes.AUTHORIZATION_CODE)) {
    return 'fullstack';
  }

  return type ?? 'custom';
}

/**
 * Reports whether the application is a machine-to-machine (`m2m`) application, preferring the
 * explicit type and falling back to config-shape inference for legacy/custom applications.
 */
export function isM2MApplication(type: ApplicationType | undefined, oauth2Config?: OAuth2Config): boolean {
  return resolveApplicationType(type, oauth2Config) === 'm2m';
}

/**
 * Reports whether `client_credentials` is the only configured grant type, mirroring the backend's
 * `isM2MGrantSet`. Used to exclude m2m-shaped configurations from confidential, non-redirect
 * (flow-native) eligibility checks for `fullstack`/`custom`/`mcp` applications.
 */
export function isClientCredentialsOnlyGrantSet(grantTypes: readonly string[] | undefined): boolean {
  return (grantTypes ?? []).length === 1 && grantTypes?.[0] === OAuth2GrantTypes.CLIENT_CREDENTIALS;
}
