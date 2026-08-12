// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import nonEmptyRowKeys from './nonEmptyRowKeys';
import type {AllowedOriginDraftRow} from '../models/allowedOriginRow';

/**
 * Builds a stable key for a set of rows, used to compare a draft against its saved baseline. The key
 * covers each row's type as well as its value, so changing only a row's type counts as a change.
 *
 * @param rows - The rows to key
 * @returns A stable string key derived from the non-empty rows
 */
export default function baselineKey(rows: AllowedOriginDraftRow[]): string {
  return JSON.stringify(nonEmptyRowKeys(rows));
}
