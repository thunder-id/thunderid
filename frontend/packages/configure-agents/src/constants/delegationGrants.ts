// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '@thunderid/configure-applications';
/**
 * authorization_code, ciba, and refresh_token only make sense once the agent can act on behalf
 * of a user; they stay visible in grant-type pickers but are locked (unselectable) until
 * Delegated mode is turned on.
 */
export const DELEGATED_ONLY_GRANTS: string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.CIBA,
  OAuth2GrantTypes.REFRESH_TOKEN,
];
