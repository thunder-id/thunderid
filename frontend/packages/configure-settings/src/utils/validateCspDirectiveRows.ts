// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CSP_NONE_SOURCE} from './cspDirectiveRows';
import type {CspScopeRow} from '../models/cspDirectiveRow';

/**
 * A blocking problem with a directive the admin set.
 *
 * @public
 */
export type CspDirectiveRowError = 'invalidSource' | 'noSources' | 'noneWithOthers';

/**
 * The English copy for each issue code, used as the fallback when resolving
 * `settings:csp.validation.<code>`.
 *
 * @public
 */
export const CspDirectiveRowIssueFallbacks: Record<CspDirectiveRowError, string> = {
  invalidSource: "A source can't contain a semicolon or comma.",
  noSources: 'Add at least one source, or reset the directive to its default.',
  noneWithOthers: "'none' can't be combined with other sources.",
};

/**
 * Reports whether the backend would reject a source.
 *
 * @param source - A single source
 * @returns Whether the source holds a separator character
 *
 * @public
 */
export function isInvalidCspSource(source: string): boolean {
  return /[\s;,]/.test(source);
}

/**
 * Validates the directives the admin set.
 *
 * @param rows - Rows of one or more scopes
 * @returns The errors, keyed by draft row id
 *
 * @public
 */
export default function validateCspDirectiveRows(rows: CspScopeRow[]): Record<string, CspDirectiveRowError> {
  const errors: Record<string, CspDirectiveRowError> = {};

  rows.forEach((row) => {
    if (row.draftId === undefined || row.override === undefined) {
      return;
    }
    const sources = row.override;
    if (sources.some(isInvalidCspSource)) {
      errors[row.draftId] = 'invalidSource';
    } else if (sources.length === 0 && row.base.length > 0) {
      errors[row.draftId] = 'noSources';
    } else if (sources.includes(CSP_NONE_SOURCE) && sources.length > 1) {
      errors[row.draftId] = 'noneWithOthers';
    }
  });

  return errors;
}
