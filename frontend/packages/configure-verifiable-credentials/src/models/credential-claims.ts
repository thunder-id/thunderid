// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ClaimMapping, VerifiableCredential} from './vc';

let rowCounter = 0;

/** An editable claim row: the attribute name and its display name. */
export interface ClaimRow {
  id: string;
  name: string;
  displayName: string;
}

/** emptyClaimRow returns a fresh, blank claim row with a stable local id. */
export function emptyClaimRow(): ClaimRow {
  rowCounter += 1;
  return {id: `claim-${rowCounter}`, name: '', displayName: ''};
}

/** credentialToClaimRows builds editor rows from a credential's claims. */
export function credentialToClaimRows(credential?: VerifiableCredential): ClaimRow[] {
  const claims = credential?.claims ?? [];
  if (claims.length === 0) {
    return [emptyClaimRow()];
  }
  return claims.map((c: ClaimMapping): ClaimRow => {
    rowCounter += 1;
    return {id: `claim-${rowCounter}`, name: c.name, displayName: c.displayName ?? ''};
  });
}

/**
 * Claim names that cannot be selectively disclosed. iss, nbf, exp, cnf, vct and status must not
 * appear in the Disclosures per SD-JWT VC; _sd, _sd_alg and '...' are structural to SD-JWT itself.
 * sub and iat may be disclosed per the specification, but this issuer always writes both into the
 * credential payload, so disclosing them would collide. Kept in step with the server-side set.
 */
export const RESERVED_CLAIM_NAMES: readonly string[] = [
  'iss',
  'nbf',
  'exp',
  'cnf',
  'vct',
  'status',
  '_sd',
  '_sd_alg',
  '...',
  'sub',
  'iat',
];

/** The reason a claim row's name is invalid, keyed by row id. */
export type ClaimNameError = 'duplicate' | 'reserved';

/**
 * findClaimNameErrors flags rows whose claim name is reserved or repeats an earlier row. Names are
 * compared case-sensitively because they become JSON keys in the issued credential. Blank rows are
 * ignored, since they are dropped before the request is built.
 */
export function findClaimNameErrors(rows: ClaimRow[]): Record<string, ClaimNameError> {
  const errors: Record<string, ClaimNameError> = {};
  const seen = new Set<string>();
  rows.forEach((row: ClaimRow): void => {
    const name = row.name.trim();
    if (name === '') {
      return;
    }
    if (RESERVED_CLAIM_NAMES.includes(name)) {
      errors[row.id] = 'reserved';
      return;
    }
    if (seen.has(name)) {
      errors[row.id] = 'duplicate';
      return;
    }
    seen.add(name);
  });
  return errors;
}

/** claimRowsToRequest maps editor rows to the API claims array, dropping unnamed rows. */
export function claimRowsToRequest(rows: ClaimRow[]): ClaimMapping[] {
  return rows
    .filter((r: ClaimRow): boolean => r.name.trim() !== '')
    .map(
      (r: ClaimRow): ClaimMapping => ({
        name: r.name.trim(),
        displayName: r.displayName.trim() || undefined,
      }),
    );
}
