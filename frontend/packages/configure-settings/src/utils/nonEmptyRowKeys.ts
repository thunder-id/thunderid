// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isRowEmpty, rowKey} from './allowedOriginRows';
import type {AllowedOriginDraftRow} from '../models/allowedOriginRow';

/**
 * Keys the non-empty rows for comparison. Row ids are deliberately excluded, so two lists holding
 * the same entries compare equal even though their rows were minted separately.
 *
 * @param rows - The rows to key
 * @returns One key per non-empty row, in order
 */
export default function nonEmptyRowKeys(rows: AllowedOriginDraftRow[]): string[] {
  return rows.filter((row) => !isRowEmpty(row)).map(rowKey);
}
