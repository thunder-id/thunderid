// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {CreateVerifiablePresentationRequest} from './presentation-requests';
import type {VerifiablePresentation} from './vp';

export type ClaimRequirement = 'mandatory' | 'optional';

let idCounter = 0;
const nextClaimId = (): string => {
  idCounter += 1;
  return `claim-${idCounter}`;
};

/**
 * A single requested claim in the unified claim editor — combining requirement
 * and value-constraint settings that the API models as separate
 * `mandatory_claims`/`optional_claims`/`claim_values`. `id` is a client-side key
 * for stable rendering and is not sent to the API.
 */
export interface ClaimRow {
  id: string;
  name: string;
  requirement: ClaimRequirement;
  values: string[];
}

export const emptyClaimRow = (): ClaimRow => ({
  id: nextClaimId(),
  name: '',
  requirement: 'mandatory',
  values: [],
});

type ClaimFields = Pick<VerifiablePresentation, 'mandatoryClaims' | 'optionalClaims' | 'claimValues'>;

/**
 * findDuplicateClaimNames flags rows whose claim name repeats an earlier row. A name may appear
 * only once across the whole editor, since a claim cannot be both mandatory and optional. Blank
 * rows are ignored, as they are dropped before the request is built.
 */
export function findDuplicateClaimNames(rows: ClaimRow[]): Record<string, true> {
  const duplicates: Record<string, true> = {};
  const seen = new Set<string>();
  rows.forEach((row: ClaimRow): void => {
    const name = row.name.trim();
    if (name === '') {
      return;
    }
    if (seen.has(name)) {
      duplicates[row.id] = true;
      return;
    }
    seen.add(name);
  });
  return duplicates;
}

/** Builds editor rows from a stored definition (edit mode), preserving order and de-duplicating. */
export function definitionToClaimRows(vp?: ClaimFields): ClaimRow[] {
  if (!vp) {
    return [];
  }
  const mandatory = new Set(vp.mandatoryClaims ?? []);
  const values = vp.claimValues ?? {};

  const ordered: string[] = [];
  const seen = new Set<string>();
  const push = (name: string): void => {
    if (name && !seen.has(name)) {
      seen.add(name);
      ordered.push(name);
    }
  };
  (vp.mandatoryClaims ?? []).forEach(push);
  (vp.optionalClaims ?? []).forEach(push);
  Object.keys(values).forEach(push);

  return ordered.map((name) => ({
    id: nextClaimId(),
    name,
    requirement: mandatory.has(name) ? 'mandatory' : 'optional',
    values: values[name] ?? [],
  }));
}

/** Converts editor rows into the API request claim fields. */
export function claimRowsToRequest(
  rows: ClaimRow[],
): Pick<CreateVerifiablePresentationRequest, 'mandatoryClaims' | 'optionalClaims' | 'claimValues'> {
  const mandatory: string[] = [];
  const optional: string[] = [];
  const claimValues: Record<string, string[]> = {};

  rows.forEach((row) => {
    const name = row.name.trim();
    if (name === '') {
      return;
    }
    if (row.requirement === 'mandatory') {
      mandatory.push(name);
    } else {
      optional.push(name);
    }
    const vals = row.values.map((v) => v.trim()).filter(Boolean);
    if (vals.length > 0) {
      claimValues[name] = vals;
    }
  });

  return {
    mandatoryClaims: mandatory,
    optionalClaims: optional,
    claimValues: Object.keys(claimValues).length > 0 ? claimValues : undefined,
  };
}
