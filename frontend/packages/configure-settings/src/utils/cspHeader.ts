// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CSP_BASELINE, rowSources} from './cspDirectiveRows';
import type {CspScopeRow} from '../models/cspDirectiveRow';

/**
 * Builds the header browsers receive for a scope from its rows, in the backend's order: baseline
 * directives first, then the rest sorted by name. The per-request nonce is left out, since it differs on
 * every response.
 *
 * @param rows - The rows of the scope
 * @param reportOnly - Whether the policy is report-only
 * @param reportUri - The effective report URI, if any
 * @returns The header line, name included
 *
 * @public
 */
export default function buildCspHeader(rows: CspScopeRow[], reportOnly: boolean, reportUri?: string): string {
  const baselineNames: string[] = CSP_BASELINE.map(([name]) => name);
  const ordered = [
    ...baselineNames.flatMap((name) => rows.filter((row) => row.directive === name)),
    ...rows
      .filter((row) => !baselineNames.includes(row.directive))
      .sort((a, b) => a.directive.localeCompare(b.directive)),
  ];
  const directives = ordered
    .map((row) => [row.directive, rowSources(row)] as const)
    .filter(([, sources]) => sources.length > 0)
    .map(([name, sources]) => `${name} ${sources.join(' ')}`);

  if (reportUri) {
    directives.push(`report-uri ${reportUri}`);
  }

  const headerName = reportOnly ? 'Content-Security-Policy-Report-Only' : 'Content-Security-Policy';
  return `${headerName}: ${directives.join('; ')}`;
}
