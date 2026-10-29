// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '@thunderid/configure-applications';

/**
 * The grant types that let an agent act on behalf of a signed-in user. An agent holding any of
 * them is delegated, the same rule the server applies when it checks an agent's grants.
 */
export const DELEGATION_GRANTS: string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.CIBA,
  OAuth2GrantTypes.TOKEN_EXCHANGE,
];

/** The grant types turning Delegated mode on gives the agent, as the server does when none are asked for. */
export const DEFAULT_DELEGATED_GRANTS: string[] = [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN];

/**
 * Every grant type that only makes sense once the agent can act on behalf of a user. They stay
 * visible in grant-type pickers but are locked (unselectable) until Delegated mode is turned on.
 */
export const DELEGATED_ONLY_GRANTS: string[] = [
  OAuth2GrantTypes.AUTHORIZATION_CODE,
  OAuth2GrantTypes.CIBA,
  OAuth2GrantTypes.REFRESH_TOKEN,
  OAuth2GrantTypes.TOKEN_EXCHANGE,
];
