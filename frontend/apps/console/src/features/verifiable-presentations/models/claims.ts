/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import type {CreateVerifiablePresentationRequest} from './requests';
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
