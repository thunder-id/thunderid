// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * An OpenID4VP presentation definition managed in the console. The verifier
 * identity is deployment-level configuration shared by all definitions, while
 * the trusted issuer certificates belong to each definition.
 */
export interface VerifiablePresentation {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  /** The project this definition belongs to. */
  projectId?: string;
  name?: string;
  description?: string;
  vct: string;
  format: string;
  mandatoryClaims?: string[];
  optionalClaims?: string[];
  /**
   * Per-claim value constraints (claim path -> allowed values). When a
   * constrained claim is disclosed, its value must match one of these.
   */
  claimValues?: Record<string, string[]>;
  /**
   * When true, the issuer's x5c chain must verify against one of the trusted
   * issuer certificates of this definition. Omitted means not enforced.
   */
  enforceTrustedIssuer?: boolean;
  /**
   * Names of the trusted issuer certificates registered on this definition.
   * The certificates themselves are never returned.
   */
  trustedAuthorities?: string[];
}

/**
 * Minimal projection returned by the list endpoint — only the fields the
 * management table renders. Use VerifiablePresentation for the detail view.
 */
export interface VerifiablePresentationSummary {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  /** The project this definition belongs to. */
  projectId?: string;
  name?: string;
  vct: string;
  format: string;
}

/** The list endpoint returns a plain array of presentation definition summaries. */
export type VPListResponse = VerifiablePresentationSummary[];

/** Response of initiating an OpenID4VP verification transaction. */
export interface InitiateVerificationResponse {
  txn_id: string;
  wallet_url: string;
  status_url?: string;
  expires_at?: string;
}

/** Status of an OpenID4VP verification transaction. */
export interface VerificationStatusResponse {
  status: string;
  result_token?: string;
  error?: string;
}
