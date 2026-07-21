/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

/**
 * Application template constants.
 */
const TemplateConstants = {
  /**
   * Template modifier suffix for embedded (inbuilt) approach.
   * Appended to technology template IDs when INBUILT approach is selected.
   */
  EMBEDDED_SUFFIX: '-embedded',

  /**
   * Template ID for the MCP Client application template. Used to branch the
   * creation wizard (Name & Type step) and the edit page's MCP-native tab set.
   */
  MCP_CLIENT_TEMPLATE_ID: 'mcp-client',

  /**
   * Grant types offered to MCP Client applications on the Advanced tab, regardless of
   * what the OIDC discovery document additionally advertises.
   */
  MCP_CLIENT_ALLOWED_GRANT_TYPES: ['authorization_code', 'refresh_token', 'client_credentials'],
} as const;

export default TemplateConstants;
