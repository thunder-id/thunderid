// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ScopeClaims} from '../models/oauth';

interface ScopeClaimsHolder {
  type: string;
  config?: {scopeClaims?: ScopeClaims};
}

const getScopeNames = (configs?: ScopeClaimsHolder[]): string =>
  Object.keys(configs?.find((config) => config.type === 'oauth2')?.config?.scopeClaims ?? {})
    .sort()
    .join(' ');

/**
 * Whether the OAuth scope-to-claims mapping gained or lost a scope. Deep comparisons that treat an
 * empty list as absent miss a scope with no attributes, yet an empty scope is a different mapping
 * from a missing one.
 *
 * @param edited - The edited inbound auth configs.
 * @param saved - The inbound auth configs as currently saved.
 */
export default function hasScopeNamesChanged(
  edited: ScopeClaimsHolder[] | undefined,
  saved: ScopeClaimsHolder[] | undefined,
): boolean {
  return getScopeNames(edited) !== getScopeNames(saved);
}
