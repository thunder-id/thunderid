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
 * Represents a processed token response with standardized camelCase properties.
 * This interface provides a clean, normalized structure for token data after
 * processing the raw response from the authentication server.
 *
 * The properties in this interface follow JavaScript/TypeScript naming conventions
 * and provide a more ergonomic API for client applications.
 */
export interface TokenResponse {
  /**
   * The bearer token used for authenticating API requests.
   * This token should be included in the Authorization header
   * of subsequent API requests.
   */
  accessToken: string;

  /**
   * Unix timestamp (in seconds) when the token was created.
   * Used in combination with expiresIn to determine when
   * the token needs to be refreshed.
   */
  createdAt: number;

  /**
   * Duration in seconds until the access token expires.
   * Applications should refresh the token before this time
   * to maintain uninterrupted access.
   */
  expiresIn: string;

  /**
   * JSON Web Token (JWT) containing user identity information.
   * This token can be decoded to access user claims and metadata
   * without additional server requests.
   */
  idToken: string;

  /**
   * Token used to obtain new access tokens without re-authentication.
   * Store this securely as it enables long-term access to the user's
   * account through the refresh flow.
   */
  refreshToken: string;

  /**
   * Space-separated list of OAuth scopes granted to the application.
   * These scopes determine what resources and actions the application
   * has permission to access.
   */
  scope: string;

  /**
   * The type of token issued, typically "Bearer".
   * This indicates how the token should be used in
   * API request Authorization headers.
   */
  tokenType: string;
}

/**
 * Represents the raw token response received directly from the authentication server.
 * This interface maintains the original snake_case property names as received in
 * the server response before processing into the standardized TokenResponse format.
 *
 * The properties in this interface exactly match the OAuth2/OIDC server response
 * format before any transformation or normalization is applied.
 */
export interface AccessTokenApiResponse {
  /**
   * Raw access token string from the server.
   * This is the bearer token in its original format
   * before any processing or validation.
   */
  access_token: string;

  /**
   * Server-provided creation timestamp in Unix seconds.
   * Used to track when the token was originally issued
   * and calculate absolute expiration time.
   */
  created_at: number;

  /**
   * Raw expiration time in seconds.
   * Indicates how long the access token will be valid
   * from the time it was issued.
   */
  expires_in: string;

  /**
   * Raw ID token string containing encoded user information.
   * This JWT can be decoded to access standardized claims
   * about the authenticated user.
   */
  id_token: string;

  /**
   * Raw refresh token string from the server.
   * Used in its original format to request new access tokens
   * when they expire.
   */
  refresh_token: string;

  /**
   * Raw space-separated scope string defining access permissions.
   * Lists the OAuth scopes that were granted during the
   * authorization process.
   */
  scope: string;

  /**
   * Raw token type identifier from the server.
   * Typically "Bearer", indicating how the token should
   * be used in API requests.
   */
  token_type: string;
}

/**
 * Interface for the standard (required) claims of an ID Token payload.
 */
export interface KnownIdToken {
  /**
   * The audience for which this token is intended.
   */
  aud: string | string[];

  /**
   * The email of the user.
   */
  email?: string;

  /**
   * The issuer identifier for the issuer of the response.
   */
  iss: string;

  /**
   * The unique human readable slug of the organization to which the user belongs.
   */
  org_handle?: string;

  /**
   * The unique identifier of the organization to which the user belongs.
   */
  org_id?: string;

  /**
   * The human readable name of the organization to which the user belongs.
   */
  org_name?: string;

  /**
   * The username the user prefers to be called.
   */
  preferred_username?: string;

  /**
   * The unique identifier of the user to whom the ID token belongs.
   */
  sub: string;

  /**
   * The tenant domain of the user.
   */
  tenant_domain?: string;
}

/**
 * Interface for ID Token payload including custom claims.
 */
export interface IdToken extends KnownIdToken {
  /**
   * Other custom claims.
   */
  [claim: string]: any;
}

export interface TokenExchangeRequestConfig {
  attachToken: boolean;
  data: any;
  id: string;
  returnsSession: boolean;
  shouldReplayAfterRefresh?: boolean;
  signInRequired: boolean;
  tokenEndpoint?: string;
}
