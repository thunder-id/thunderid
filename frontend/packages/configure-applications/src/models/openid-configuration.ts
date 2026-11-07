// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * The fields of the runtime's OpenID Connect discovery document
 * (`/.well-known/openid-configuration`) that the console reads to offer an application's OAuth
 * settings.
 *
 * @public
 * @remarks
 * The document is served by the runtime the applications call, which can differ from the server
 * the console signs in to. Only the fields the console reads are declared.
 */
export interface OpenIDConfiguration {
  /**
   * Grant types the runtime's token endpoint serves.
   * @example ["authorization_code", "refresh_token"]
   */
  grant_types_supported?: string[];
  /**
   * Response types the runtime's authorization endpoint serves.
   * @example ["code"]
   */
  response_types_supported?: string[];
  /**
   * Client authentication methods the runtime's token endpoint accepts.
   * @example ["client_secret_basic", "private_key_jwt", "none"]
   */
  token_endpoint_auth_methods_supported?: string[];
  /**
   * Authentication context class references the runtime offers. Left out when it offers none.
   * @example ["mfa"]
   */
  acr_values_supported?: string[];
}
