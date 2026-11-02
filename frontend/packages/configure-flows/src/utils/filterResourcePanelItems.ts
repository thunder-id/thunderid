// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import getResourceKind from './getResourceKind';
import type {ResourcePanelListItem} from './toResourcePanelItems';
import type {Resource} from '../models/resources';

/**
 * Common capability synonyms mapped to terms that appear in resource labels,
 * descriptions, and types, so searches like "MFA" or "social" surface the
 * related resources.
 */
const SYNONYMS: Record<string, string[]> = {
  '2fa': ['otp', 'passcode', 'factor'],
  mfa: ['otp', 'passcode', 'factor'],
  passwordless: ['passkey', 'magic link', 'wallet'],
  social: ['google', 'github', 'oauth', 'oidc', 'federated'],
  sso: ['oauth', 'oidc', 'federated'],
};

function buildResourceHaystack(resource: Resource): string {
  return [
    resource.display?.label,
    resource.display?.description,
    resource.type?.replaceAll('_', ' '),
    resource.category?.replaceAll('_', ' '),
    getResourceKind(resource),
  ]
    .filter(Boolean)
    .join(' ')
    .toLowerCase();
}

/**
 * Filters resource panel items by a search query.
 *
 * Every whitespace-separated term of the query must match the resource's label,
 * description, type, category, or kind — either directly or via a capability
 * synonym (e.g. "mfa" matches OTP resources).
 *
 * @param items - Panel list items to filter.
 * @param query - Search query. A blank query returns the items unchanged.
 * @returns Filtered panel list items.
 */
function filterResourcePanelItems(items: ResourcePanelListItem[], query: string): ResourcePanelListItem[] {
  const terms: string[] = query.trim().toLowerCase().split(/\s+/).filter(Boolean);

  if (terms.length === 0) {
    return items;
  }

  return items.filter((item: ResourcePanelListItem) => {
    const haystack: string = buildResourceHaystack(item.resource);

    return terms.every((term: string) => {
      const candidates: string[] = [term, ...(SYNONYMS[term] ?? [])];
      return candidates.some((candidate: string) => haystack.includes(candidate));
    });
  });
}

export default filterResourcePanelItems;
