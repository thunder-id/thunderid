// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {validateAllowedOriginRows, type AllowedOriginDraftRow} from '@thunderid/configure-settings';

/**
 * Reports whether any Configuration step CORS row would be rejected, so the wizard can block Create
 * rather than dropping the entry on the way to the deployment's allow-list.
 *
 * No existing-entry set is passed: a row that repeats an origin the deployment already allows is a
 * no-op for `mergeCorsOrigins`, not a mistake the admin has to correct before continuing.
 */
export default function hasInvalidCorsRows(rows: AllowedOriginDraftRow[]): boolean {
  return Object.keys(validateAllowedOriginRows(rows).errors).length > 0;
}
