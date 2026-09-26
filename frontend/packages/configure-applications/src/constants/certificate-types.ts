// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Certificate type constants for application configuration.
 */
const CertificateTypes = {
  /**
   * No certificate configured
   */
  NONE: 'NONE',
  /**
   * JWKS (JSON Web Key Set) certificate
   */
  JWKS: 'JWKS',
  /**
   * JWKS URI (URL pointing to JWKS)
   */
  JWKS_URI: 'JWKS_URI',
} as const;

export default CertificateTypes;
