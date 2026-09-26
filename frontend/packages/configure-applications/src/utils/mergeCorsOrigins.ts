// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  isRowEmpty,
  rowKey,
  toAllowedOrigins,
  toRows,
  validateAllowedOriginRows,
  type AllowedOrigin,
  type AllowedOriginDraftRow,
  type CorsValue,
} from '@thunderid/configure-settings';

/**
 * Builds the CORS PUT payload for adding the Configuration step's origins to the deployment's
 * writable allow-list, without disturbing existing entries. Additions that are blank or already
 * present (writable or read-only) are skipped rather than duplicated or overwritten. Entries are
 * compared by type as well as value, so an added pattern never collides with a literal of the same
 * text.
 *
 * Invalid additions are also skipped, but only as a backstop: the Configuration step blocks Create
 * while any row is invalid, so nothing the admin typed should reach here and disappear.
 */
export default function mergeCorsOrigins(
  writable: AllowedOrigin[],
  readOnly: AllowedOrigin[],
  additions: AllowedOriginDraftRow[],
): CorsValue {
  const existing = new Set([...toRows(writable), ...toRows(readOnly)].map(rowKey));
  const merged: AllowedOrigin[] = [...writable];

  additions.forEach((row) => {
    const key = rowKey(row);
    if (isRowEmpty(row) || existing.has(key)) return;
    if (Object.keys(validateAllowedOriginRows([row]).errors).length > 0) return;
    existing.add(key);
    merged.push(...toAllowedOrigins([row]));
  });

  return {allowedOrigins: merged};
}
