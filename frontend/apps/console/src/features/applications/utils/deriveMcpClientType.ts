/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
