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
