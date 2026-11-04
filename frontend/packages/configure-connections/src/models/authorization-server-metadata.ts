// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The fields of the runtime's OAuth 2.0 authorization server metadata (RFC 8414,
 * `/.well-known/oauth-authorization-server`) that the console reads to tell which capabilities
 * the runtime serves.
 *
 * @public
 * @remarks
 * The document is served by the runtime the applications call, which can differ from the server
 * the console signs in to. Only the fields the console reads are declared.
 */
export interface AuthorizationServerMetadata {
  /**
   * Grant types the runtime's token endpoint serves. The runtime always sends this field.
   * @example ["authorization_code", "urn:ietf:params:oauth:grant-type:token-exchange"]
   */
  grant_types_supported?: string[];
  /**
   * Authorization grant profiles the runtime accepts. Left out when the runtime accepts none.
   * @example ["urn:ietf:params:oauth:grant-profile:id-jag"]
   */
  authorization_grant_profiles_supported?: string[];
}
