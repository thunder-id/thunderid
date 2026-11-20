// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * A trusted issuer certificate entry sent with a presentation definition. An
 * entry without a certificate keeps the certificate already stored under that name.
 */
export interface TrustedAuthorityInput {
  /** Label of the certificate, unique within the definition. */
  name: string;
  /** PEM-encoded X.509 certificate. Required when the entry is newly added. */
  certificate?: string;
}

/** Request body for creating a presentation definition. */
export interface CreateVerifiablePresentationRequest {
  handle: string;
  /**
   * The organization unit this definition belongs to. A definition in a project leaves it out to be placed in
   * the system organization unit, or names one of the project's.
   */
  ouId?: string;
  /** The project this definition belongs to. Required on the control plane. */
  projectId?: string;
  name?: string;
  description?: string;
  vct: string;
  format?: string;
  mandatoryClaims?: string[];
  optionalClaims?: string[];
  claimValues?: Record<string, string[]>;
  enforceTrustedIssuer?: boolean;
  trustedAuthorities?: TrustedAuthorityInput[];
  /** The mandatory claim that identifies the holder at sign-in. Omitted uses the credential `sub`. */
  subjectClaim?: string;
}

/** Request body for updating a presentation definition. */
export type UpdateVerifiablePresentationRequest = CreateVerifiablePresentationRequest;
