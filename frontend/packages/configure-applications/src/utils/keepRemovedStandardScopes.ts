// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import TokenConstants from '../constants/token-constants';
import type {ScopeClaims} from '../models/oauth';

/**
 * Keeps a standard OIDC scope removed in this edit as an empty list. The server falls back to the
 * standard claims for a standard scope the mapping leaves out, so only an empty list stops it
 * releasing attributes.
 *
 * @param edited - The edited inbound auth configs about to be saved.
 * @param saved - The inbound auth configs as currently saved.
 * @returns The edited configs, with each removed standard scope mapped to an empty list.
 */
export default function keepRemovedStandardScopes<T extends {type: string; config?: {scopeClaims?: ScopeClaims}}>(
  edited: T[],
  saved: T[] | undefined,
): T[] {
  const savedScopeClaims = saved?.find((config) => config.type === 'oauth2')?.config?.scopeClaims ?? {};

  return edited.map((config) => {
    const scopeClaims = config.type === 'oauth2' ? config.config?.scopeClaims : undefined;
    if (!scopeClaims) return config;

    const removed = TokenConstants.STANDARD_OIDC_SCOPES.filter(
      (scope) => scope in savedScopeClaims && !(scope in scopeClaims),
    );
    if (removed.length === 0) return config;

    return {
      ...config,
      config: {
        ...config.config,
        scopeClaims: {...scopeClaims, ...Object.fromEntries(removed.map((scope) => [scope, []]))},
      },
    };
  });
}
