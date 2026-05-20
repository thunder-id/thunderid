/**
 * Copyright (c) 2020, WSO2 LLC. (https://www.wso2.com). All Rights Reserved.
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
 * Comprehensive OpenID Connect (OIDC) Provider Metadata.
 * This interface represents the complete set of configuration metadata
 * that an OpenID Provider (OP) may publish at its discovery endpoint.
 *
 * @remarks
 * The metadata is organized into several categories:
 * 1. Core Endpoints - Essential URLs for OIDC operations
 * 2. Capability Indicators - Supported features and algorithms
 * 3. Claims and Scopes - Available user information
 * 4. Security Settings - Authentication and encryption options
 * 5. UI/UX Configuration - Display and localization preferences
 *
 * All fields are optional as per the OIDC Discovery specification,
 * allowing for flexible provider implementations.
 *
 * @see {@link https://openid.net/specs/openid-connect-discovery-1_0.html OpenID Connect Discovery Specification}
 *
 * @example
 * ```typescript
 * const config: OIDCProviderMetadata = {
 *   issuer: "https://accounts.example.com",
 *   authorization_endpoint: "https://accounts.example.com/auth",
 *   token_endpoint: "https://accounts.example.com/token",
 *   userinfo_endpoint: "https://accounts.example.com/userinfo",
 *   jwks_uri: "https://accounts.example.com/jwks.json"
 * };
 * ```
 */
export interface OIDCDiscoveryApiResponse extends OIDCDiscoveryEndpointsApiResponse {
  /**
   * Supported Authentication Context Class References.
   * Indicates the OP's ability to satisfy specific authentication requirements.
   */
  acr_values_supported?: string[];

  /**
   * Boolean value specifying whether the OP can pass a sid (session ID) Claim in the Logout Token to
   * identify the RP session with the OP.
   */
  backchannel_logout_session_supported?: boolean;

  /**
   * Boolean value specifying whether the OP supports back-channel logout, with true indicating support.
   * If omitted, the default value is false.
   */
  backchannel_logout_supported?: boolean;

  /**
   * JSON array containing a list of the Claim Types that the OpenID Provider supports.
   */
  claim_types_supported?: string[];

  /**
   * Languages and scripts supported for values in Claims being returned, represented as a JSON array
   * of BCP47 [RFC5646] language tag values. Not all languages and scripts are necessarily
   * supported for all Claim values.
   */
  claims_locales_supported?: string[];

  /**
   *  Boolean value specifying whether the OP supports use of the claims parameter,
   * with true indicating support. If omitted, the default value is false.
   */
  claims_parameter_supported?: boolean;

  /**
   * JSON array containing a list of the Claim Names of the Claims that
   * the OpenID Provider MAY be able to supply values for.
   */
  claims_supported?: string[];

  /**
   * JSON array containing a list of Proof Key for Code
   * Exchange (PKCE) [RFC7636] code challenge methods supported by this
   * authorization server.
   */
  code_challenge_methods_supported?: string[];

  /**
   * JSON array containing a list of the display parameter values that the OpenID Provider supports.
   */
  display_values_supported?: string[];

  /**
   * Supported OAuth 2.0 grant type values.
   *
   * @remarks
   * Common values include:
   * - 'authorization_code' - Standard OAuth 2.0 auth code flow
   * - 'implicit' - Implicit flow
   * - 'refresh_token' - Refresh token grant
   * - 'client_credentials' - Client credentials grant
   */
  grant_types_supported?: string[];

  /**
   * JSON array containing a list of the JWE encryption algorithms (alg values)
   * supported by the OP for the ID Token to encode the Claims in a JWT [JWT].
   */
  id_token_encryption_alg_values_supported?: string[];

  /**
   * JSON array containing a list of the JWE encryption algorithms (enc values)
   * supported by the OP for the ID Token to encode the Claims in a JWT [JWT].
   */
  id_token_encryption_enc_values_supported?: string[];

  /**
   * JSON array containing a list of the JWS signing algorithms (alg values)
   * supported by the OP for the ID Token to encode the Claims in a JWT [JWT].
   */
  id_token_signing_alg_values_supported?: string[];

  /**
   * JSON array containing a list of client authentication
   * methods supported by this introspection endpoint.
   */
  introspection_endpoint_auth_methods_supported?: string[];

  /**
   * JSON array containing a list of the JWS signing
   * algorithms ("alg" values) supported by the introspection endpoint
   * for the signature on the JWT [JWT] used to authenticate the client
   * at the introspection endpoint for the "private_key_jwt" and
   * "client_secret_jwt" authentication methods.
   */
  introspection_endpoint_auth_signing_alg_values_supported?: string[];

  /**
   * URL that the OpenID Provider provides to the person registering the Client
   * to read about the OP's requirements on how the Relying Party can use the data provided by the OP.
   */
  op_policy_uri?: string;

  /**
   * URL that the OpenID Provider provides to the person registering the Client
   * to read about OpenID Provider's terms of service.
   */
  op_tos_uri?: string;

  /**
   * JSON array containing a list of the JWE encryption algorithms (alg values)
   * supported by the OP for Request Objects.
   */
  request_object_encryption_alg_values_supported?: string[];

  /**
   * JSON array containing a list of the JWE encryption algorithms (enc values)
   * supported by the OP for Request Objects.
   */
  request_object_encryption_enc_values_supported?: string[];

  /**
   * JSON array containing a list of the JWS signing algorithms (alg values) supported by the OP for Request Objects
   */
  request_object_signing_alg_values_supported?: string[];

  /**
   * Boolean value specifying whether the OP supports use of the request parameter,
   * with true indicating support. If omitted, the default value is false.
   */
  request_parameter_supported?: boolean;

  /**
   * Boolean value specifying whether the OP supports use of the request_uri parameter,
   * with true indicating support. If omitted, the default value is true.
   */
  request_uri_parameter_supported?: boolean;

  /**
   * Boolean value specifying whether the OP requires any request_uri values used to be
   * pre-registered using the request_uris registration parameter.
   */
  require_request_uri_registration?: boolean;

  /**
   * Supported OAuth 2.0 response_mode values.
   *
   * @remarks
   * Determines how the authorization response is returned:
   * - 'query' - Parameters in URL query string
   * - 'fragment' - Parameters in URL fragment
   * - 'form_post' - Parameters via POST
   */
  response_modes_supported?: string[];

  /**
   * Supported OAuth 2.0 response_type values.
   *
   * @remarks
   * Common values include:
   * - 'code' - Authorization Code flow
   * - 'token' - Implicit flow
   * - 'id_token' - OIDC implicit flow
   * - 'code token' - Hybrid flow
   */
  response_types_supported?: string[];

  /**
   * JSON array containing a list of client authentication
   * methods supported by this revocation endpoint.
   */
  revocation_endpoint_auth_methods_supported?: string[];

  /**
   * JSON array containing a list of the JWS signing
   * algorithms ("alg" values) supported by the revocation endpoint for
   * the signature on the JWT [JWT] used to authenticate the client at
   * the revocation endpoint for the "private_key_jwt" and
   * "client_secret_jwt" authentication methods.
   */
  revocation_endpoint_auth_signing_alg_values_supported?: string[];

  /**
   * Supported OAuth 2.0 scope values.
   * Lists the permission scopes this server can handle.
   *
   * @remarks
   * Common values include:
   * - 'openid' - Required for OIDC flows
   * - 'profile' - Basic user information
   * - 'email' - User's email address
   * - 'address' - User's postal address
   * - 'phone' - User's phone number
   */
  scopes_supported?: string[];

  /**
   * URL of a page containing human-readable information that developers
   * might want or need to know when using the OpenID Provider.
   */
  service_documentation?: string;

  /**
   * Supported Subject Identifier types.
   * Defines how the OP identifies users across sessions.
   *
   * @remarks
   * Common values:
   * - 'public' - Same sub value for all clients
   * - 'pairwise' - Different sub values for different clients
   */
  subject_types_supported?: string[];

  /**
   * JSON array containing a list of Client Authentication methods supported by this Token Endpoint.
   */
  token_endpoint_auth_methods_supported?: string[];

  /**
   * JSON array containing a list of the JWS signing algorithms (alg values) supported by the Token Endpoint
   * for the signature on the JWT [JWT] used to authenticate the Client at the Token Endpoint for the
   * private_key_jwt and client_secret_jwt authentication methods.
   */
  token_endpoint_auth_signing_alg_values_supported?: string[];

  /**
   * Languages and scripts supported for the user interface,
   * represented as a JSON array of BCP47 [RFC5646] language tag values.
   */
  ui_locales_supported?: string[];

  /**
   * JSON array containing a list of the JWE [JWE] encryption algorithms (alg values)
   * [JWA] supported by the UserInfo Endpoint to encode the Claims in a JWT [JWT].
   */
  userinfo_encryption_alg_values_supported?: string[];

  /**
   * JSON array containing a list of the JWE encryption algorithms (enc values) [JWA]
   * supported by the UserInfo Endpoint to encode the Claims in a JWT [JWT]
   */
  userinfo_encryption_enc_values_supported?: string[];

  /**
   * JSON array containing a list of the JWS [JWS] signing algorithms (alg values) [JWA]
   * supported by the UserInfo Endpoint to encode the Claims in a JWT [JWT].
   */
  userinfo_signing_alg_values_supported?: string[];
}

/**
 * Essential OpenID Connect (OIDC) Provider endpoints configuration.
 * This interface represents the core set of endpoints that an OpenID Provider
 * must expose for basic OIDC functionality.
 *
 * @remarks
 * These endpoints form the foundation of OIDC operations and are organized into categories:
 * 1. Authentication Flow - Authorization and token endpoints
 * 2. User Data - UserInfo and session management
 * 3. Security - Key management and token operations
 * 4. Session Management - Logout and session state
 *
 * While all fields are optional in the interface, an OIDC Provider typically
 * implements most of these endpoints for full OIDC compliance.
 *
 * @see {@link OIDCDiscoveryApiResponse} For the complete provider metadata
 * @see {@link https://openid.net/specs/openid-connect-discovery-1_0.html#ProviderMetadata Provider Metadata Specification}
 *
 * @example
 * ```typescript
 * const endpoints: OIDCDiscoveryEndpointsApiResponse = {
 *   issuer: "https://identity.example.com",
 *   authorization_endpoint: "https://identity.example.com/oauth2/authorize",
 *   token_endpoint: "https://identity.example.com/oauth2/token",
 *   userinfo_endpoint: "https://identity.example.com/oauth2/userinfo"
 * };
 * ```
 */
export interface OIDCDiscoveryEndpointsApiResponse {
  /**
   * OAuth 2.0 Authorization Endpoint URL.
   * Used to initiate the authentication and authorization process.
   *
   * @remarks
   * The client redirects the user to this endpoint to begin the auth flow.
   * Supports various response_type values for different OAuth 2.0 flows.
   */
  authorization_endpoint?: string;

  /**
   * URL of an OP iframe that supports cross-origin communications for session state information with the RP
   * Client, using the HTML5 postMessage API.
   */
  check_session_iframe?: string;

  /**
   * URL at the OP to which an RP can perform a redirect to request that the End-User be logged out at the
   * OP.
   */
  end_session_endpoint?: string;

  /**
   * URL of the authorization server's OAuth 2.0
   * introspection endpoint.
   */
  introspection_endpoint?: string;

  // ====================================
  // Core Endpoints
  // ====================================
  /**
   * HTTPS URL that the OP asserts as its Issuer Identifier.
   * Must not contain query or fragment components.
   *
   * @remarks
   * This is a crucial identifier for the OpenID Provider and should
   * match the iss claim in issued JWT tokens.
   */
  issuer?: string;

  /**
   * JSON Web Key Set (JWKS) document URL.
   * Contains the cryptographic keys used to secure communications.
   *
   * @remarks
   * Used by clients to:
   * - Validate signatures on JWT tokens
   * - Encrypt requests to the OP
   * - Establish secure communications
   */
  jwks_uri?: string;

  // ====================================
  // Registration & Dynamic Configuration
  // ====================================
  /**
   * Dynamic Client Registration Endpoint URL.
   * Allows automated registration of OAuth 2.0 clients.
   *
   * @remarks
   * If supported, enables automated client setup and configuration.
   * May require initial authentication or access tokens.
   */
  registration_endpoint?: string;

  /**
   * URL of the authorization server's OAuth 2.0 revocation
   * endpoint.
   */
  revocation_endpoint?: string;

  /**
   * OAuth 2.0 Token Endpoint URL.
   * Used to obtain tokens using various grant types.
   *
   * @remarks
   * Clients use this endpoint to exchange authorization codes for tokens
   * and to refresh expired access tokens.
   */
  token_endpoint?: string;

  // ====================================
  // User Information & Key Management
  // ====================================
  /**
   * UserInfo Endpoint URL.
   * Returns claims about the authenticated end-user.
   *
   * @remarks
   * Requires a valid access token with appropriate scope.
   * May return claims in JWT format if signing/encryption is configured.
   */
  userinfo_endpoint?: string;
}
