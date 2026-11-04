// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes, type OAuth2Config} from '@thunderid/configure-applications';

/**
 * Applies the agent PKCE rule to a set of OAuth2 config updates. For agents, PKCE is fully derived
 * from the authorization code grant rather than set by hand, so an update that leaves the agent
 * holding that grant always requires PKCE. Updates that do not change the grant types are returned
 * unchanged.
 *
 * @param updates - The OAuth2 config updates, usually from `applyGrantTypesChange`
 * @returns The same updates, with `pkceRequired` set when the next grant types hold authorization code
 */
export default function applyAgentPkceRule(updates: Partial<OAuth2Config>): Partial<OAuth2Config> {
  if (!updates.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE)) return updates;
  return {...updates, pkceRequired: true};
}
