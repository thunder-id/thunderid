// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AttributeConfiguration} from './connection';

/**
 * A trusted issuer is a trust-only OIDC connection (`/connections/oidc`) that stores just
 * enough configuration for ThunderID to validate identity assertions issued by an external
 * IdP for ID-JAG consumption. Unlike a full OIDC connection, it carries no OAuth client
 * credentials.
 *
 * @public
 * @remarks
 * Trusted issuers are OIDC connections with `idJagEnabled` set (`true` or `false`); the
 * listing filters out plain federation OIDC connections where the field is `undefined`.
 */
export interface TrustedIssuerFormData {
  /**
   * Display name for the trusted issuer.
   * @example "Acme Corp Okta"
   */
  name: string;
  /**
   * The issuer URI from the external IdP's OpenID Connect discovery document.
   * @example "https://acme.okta.com"
   */
  issuer: string;
  /**
   * The JWKS endpoint used to validate the signature of incoming identity assertions.
   * @example "https://acme.okta.com/oauth2/v1/keys"
   */
  jwksEndpoint: string;
  /**
   * Whether ThunderID accepts and exchanges identity assertions issued by this issuer.
   * @example true
   */
  idJagEnabled: boolean;
  /**
   * Whether token exchange is enabled for this issuer.
   * @example true
   */
  tokenExchangeEnabled?: boolean;
  /**
   * The audience value ThunderID expects in subject tokens from this issuer during token exchange.
   * @example "thunderid-console"
   */
  trustedTokenAudience?: string;
  /**
   * Attribute and authorization mapping stored on the connection. Sent back unchanged on save
   * so the update does not drop it.
   */
  attributeConfiguration?: AttributeConfiguration;
}

/**
 * A trusted issuer as returned by the API, including its connection id.
 * @public
 */
export interface TrustedIssuer extends TrustedIssuerFormData {
  /**
   * The underlying OIDC connection id.
   * @example "8f14e45f-ceea-467e-b0a4-fbc3c7f8b52a"
   */
  id: string;
}
