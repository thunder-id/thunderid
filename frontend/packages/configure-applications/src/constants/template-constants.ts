// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Application template constants.
 */
const TemplateConstants = {
  /**
   * Template modifier suffix for embedded (inbuilt) approach.
   * Appended to technology template IDs when REDIRECT_BASED approach is selected.
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
