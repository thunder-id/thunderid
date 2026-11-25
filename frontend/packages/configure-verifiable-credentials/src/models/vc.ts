// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** A selectively disclosable claim mapped from the user's profile attribute. */
export interface ClaimMapping {
  name: string;
  displayName?: string;
}

/** Wallet-facing display metadata with no admin-facing equivalent (name/description come from the config itself). */
export interface CredentialDisplay {
  locale?: string;
  logoUri?: string;
}

/**
 * An OpenID4VCI credential configuration managed in the console. The handle is
 * the credential_configuration_id and the OAuth scope.
 */
export interface VerifiableCredential {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  /** The project this configuration belongs to. */
  projectId?: string;
  name?: string;
  description?: string;
  format: string;
  vct: string;
  claims?: ClaimMapping[];
  display?: CredentialDisplay;
  validitySeconds?: number;
}

/**
 * Minimal projection returned by the list endpoint — only the fields the
 * management table renders. Use VerifiableCredential for the detail view.
 */
export interface VerifiableCredentialSummary {
  id: string;
  handle: string;
  ouId: string;
  ouHandle?: string;
  /** The project this configuration belongs to. */
  projectId?: string;
  format: string;
  vct: string;
  name?: string;
}

/** The list endpoint returns a plain array of credential configuration summaries. */
export type VCListResponse = VerifiableCredentialSummary[];

/** Response of the issuer-initiated credential offer endpoint. */
export interface CredentialOfferResponse {
  credential_offer: Record<string, unknown>;
  credential_offer_uri: string;
}
