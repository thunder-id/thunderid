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
