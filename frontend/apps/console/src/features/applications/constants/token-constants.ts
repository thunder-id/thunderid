/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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
 * Token-related constants
 */
const TokenConstants = {
  /**
   * Default JWT token attributes that are always included in tokens
   */
  DEFAULT_TOKEN_ATTRIBUTES: ['aud', 'client_id', 'exp', 'grant_type', 'iat', 'iss', 'jti', 'nbf', 'scope', 'sub'],

  /**
   * Default attributes for User Info response
   */
  USER_INFO_DEFAULT_ATTRIBUTES: ['sub'],

  /**
   * Additional system attributes that can be configured as user attributes
   */
  ADDITIONAL_USER_ATTRIBUTES: ['groups', 'ouHandle', 'ouId', 'ouName', 'roles', 'userType'],

  /**
   * Supported UserInfo response types
   */
  /**
   * Supported ID token response types
   */
  ID_TOKEN_RESPONSE_TYPES: ['JWT', 'JWE', 'NESTED_JWT'],

  /**
   * Supported JWE key-management algorithms for ID token encryption
   */
  ID_TOKEN_ENCRYPTION_ALGS: ['RSA-OAEP', 'RSA-OAEP-256'],

  /**
   * Supported JWE content-encryption algorithms for ID token encryption
   */
  ID_TOKEN_ENCRYPTION_ENCS: ['A128CBC-HS256', 'A256GCM'],

  USER_INFO_RESPONSE_TYPES: ['JSON', 'JWS', 'JWE', 'NESTED_JWT'],

  /**
   * Supported JWS signing algorithms for UserInfo responses
   */
  USER_INFO_SIGNING_ALGS: ['RS256', 'RS512', 'PS256', 'ES256', 'ES384', 'ES512', 'EdDSA'],

  /**
   * Supported JWE key-management algorithms for UserInfo responses
   */
  USER_INFO_ENCRYPTION_ALGS: ['RSA-OAEP', 'RSA-OAEP-256'],

  /**
   * Supported JWE content-encryption algorithms for UserInfo responses
   */
  USER_INFO_ENCRYPTION_ENCS: ['A128CBC-HS256', 'A256GCM'],
} as const;

export default TokenConstants;
