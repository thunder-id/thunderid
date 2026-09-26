// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
   * Agent system, OU, group, and role attributes that can be added to the agent's own access token
   * (client_credentials)
   */
  ADDITIONAL_AGENT_ATTRIBUTES: ['name', 'owner', 'ouHandle', 'ouId', 'ouName', 'groups', 'roles', 'sub_type'],

  /**
   * Optional claims that can be added to the client access token (client_credentials grant),
   * where there is no end user to source attributes from.
   */
  CLIENT_TOKEN_OPTIONAL_CLAIMS: ['groups', 'ouHandle', 'ouId', 'ouName', 'roles', 'sub_type'],

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
   * Supported JWE key-management algorithms for UserInfo responses
   */
  USER_INFO_ENCRYPTION_ALGS: ['RSA-OAEP', 'RSA-OAEP-256'],

  /**
   * Supported JWE content-encryption algorithms for UserInfo responses
   */
  USER_INFO_ENCRYPTION_ENCS: ['A128CBC-HS256', 'A256GCM'],
} as const;

export default TokenConstants;
