// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {McpClientTypes} from '../models/mcp-client';
import type {McpClientType} from '../models/mcp-client';
import {OAuth2GrantTypes} from '../models/oauth';

/**
 * Derives the MCP client type from an OAuth2 configuration's granted grant types.
 *
 * A client is machine-to-machine (`m2m`) when it is granted `client_credentials`
 * without also being granted `authorization_code`. Every other combination — including
 * no grant types at all — is treated as user-delegated, since the mcp-client template
 * always issues `authorization_code` for clients acting on behalf of a signed-in user.
 *
 * @param grantTypes - The OAuth2 grant types currently configured for the client
 * @returns The derived MCP client type
 *
 * @example
 * ```ts
 * deriveMcpClientType(['client_credentials']); // 'm2m'
 * deriveMcpClientType(['authorization_code', 'refresh_token']); // 'userDelegated'
 * ```
 *
 * @public
 */
export default function deriveMcpClientType(grantTypes: string[] | undefined): McpClientType {
  const grants = grantTypes ?? [];
  const isM2m =
    grants.includes(OAuth2GrantTypes.CLIENT_CREDENTIALS) && !grants.includes(OAuth2GrantTypes.AUTHORIZATION_CODE);

  return isM2m ? McpClientTypes.M2M : McpClientTypes.USER_DELEGATED;
}
