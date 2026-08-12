// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isRowEmpty, normalizeRowValue, rowKey} from './allowedOriginRows';
import isRegexAnchored from './isRegexAnchored';
import {isValidOrigin, isValidRegex} from './origin';
import {AllowedOriginTypes, type AllowedOriginDraftRow} from '../models/allowedOriginRow';

/**
 * A blocking problem with a row.
 *
 * @public
 */
export type AllowedOriginRowError = 'invalidOrigin' | 'invalidRegex' | 'duplicate';

/**
 * A non-blocking caution about a row.
 *
 * @public
 */
export type AllowedOriginRowWarning = 'unanchoredRegex';

/**
 * The English copy for each issue code, used as the positional fallback when resolving
 * `settings:cors.validation.<code>`. Codes are resolved dynamically, so a catalog that is missing one
 * would otherwise render the raw key as helper text. Kept beside the codes so a new code cannot be
 * added without its default copy.
 *
 * @public
 */
export const AllowedOriginRowIssueFallbacks: Record<AllowedOriginRowError | AllowedOriginRowWarning, string> = {
  invalidOrigin:
    'Enter a valid origin, e.g. https://app.example.com. Paths, query strings, and fragments are not allowed.',
  invalidRegex: 'Enter a valid regular expression.',
  duplicate: 'This entry is already in the list.',
  unanchoredRegex: 'This pattern is not anchored with ^ and $, so it also matches any origin that merely contains it.',
};

/**
 * Per-row problems, keyed by row id.
 *
 * @public
 */
export interface AllowedOriginRowIssues {
  /** Problems that must be fixed before saving. */
  errors: Record<string, AllowedOriginRowError>;
  /** Cautions that do not prevent saving. */
  warnings: Record<string, AllowedOriginRowWarning>;
}

/**
 * Validates rows against the rules for their declared type, and flags patterns the matcher would
 * apply more broadly than they look. Returns codes rather than messages so each surface can resolve
 * them in its own namespace.
 *
 * @param rows - The rows to validate
 * @param existingKeys - Keys of entries the rows must not collide with, such as the read-only layer
 * @returns The blocking errors and non-blocking warnings, keyed by row id
 *
 * @public
 */
export default function validateAllowedOriginRows(
  rows: AllowedOriginDraftRow[],
  existingKeys?: ReadonlySet<string>,
): AllowedOriginRowIssues {
  const seen = new Set<string>();
  const repeated = new Set<string>();
  rows.forEach((row) => {
    if (!isRowEmpty(row)) {
      const key = rowKey(row);
      if (seen.has(key)) {
        repeated.add(key);
      }
      seen.add(key);
    }
  });

  const errors: Record<string, AllowedOriginRowError> = {};
  const warnings: Record<string, AllowedOriginRowWarning> = {};

  rows.forEach((row) => {
    if (isRowEmpty(row)) {
      return;
    }
    const value = normalizeRowValue(row);
    const key = rowKey(row);
    const isRegex = row.type === AllowedOriginTypes.REGEX;
    const compiles = isRegex && isValidRegex(value);

    if (isRegex && !compiles) {
      errors[row.id] = 'invalidRegex';
    } else if (!isRegex && !isValidOrigin(value)) {
      errors[row.id] = 'invalidOrigin';
    } else if (repeated.has(key) || existingKeys?.has(key)) {
      errors[row.id] = 'duplicate';
    }

    // Anchoring is advisory and independent of the duplicate check, but there is nothing useful to
    // say about the anchors of a pattern that does not compile.
    if (compiles && !isRegexAnchored(value)) {
      warnings[row.id] = 'unanchoredRegex';
    }
  });

  return {errors, warnings};
}
