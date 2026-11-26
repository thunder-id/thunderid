// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  CSP_DEFAULT_SCOPE,
  type CspDirectiveDraftRow,
  type CspDraftScopes,
  type CspScopeRow,
} from '../models/cspDirectiveRow';
import type {CspConfigResponse, CspDirectives, CspValue} from '../models/responses';

/**
 * The deny-first baseline, in header order. Mirrors the backend baseline in `internal/system/csp`.
 *
 * @public
 */
export const CSP_BASELINE: readonly [string, readonly string[]][] = [
  ['default-src', ["'none'"]],
  ['script-src', ["'self'"]],
  ['style-src', ["'self'"]],
  ['img-src', ["'self'"]],
  ['connect-src', ["'self'"]],
  ['frame-ancestors', ["'none'"]],
  ['base-uri', ["'none'"]],
  ['form-action', ["'self'"]],
];

/**
 * The directives offered when adding a row.
 *
 * @public
 */
export const CSP_DIRECTIVES: readonly string[] = [
  'default-src',
  'script-src',
  'script-src-elem',
  'script-src-attr',
  'style-src',
  'style-src-elem',
  'style-src-attr',
  'img-src',
  'font-src',
  'connect-src',
  'media-src',
  'object-src',
  'frame-src',
  'child-src',
  'worker-src',
  'manifest-src',
  'frame-ancestors',
  'base-uri',
  'form-action',
];

/**
 * The source that allows nothing. It cannot be combined with other sources.
 *
 * @public
 */
export const CSP_NONE_SOURCE = "'none'";

let rowCounter = 0;

/**
 * Builds a new draft row with a fresh client-only id.
 *
 * @param directive - The directive name
 * @param sources - The directive's sources
 * @returns A new row
 *
 * @public
 */
export function createCspRow(directive: string, sources: string[] = []): CspDirectiveDraftRow {
  rowCounter += 1;
  return {id: `csp-directive-${rowCounter}`, directive, sources};
}

/**
 * Splits typed or pasted text into sources.
 *
 * @param text - Text holding one or more sources separated by whitespace
 * @returns The non-empty sources, in order
 *
 * @public
 */
export function parseSources(text: string): string[] {
  return text.split(/\s+/).filter((source) => source !== '');
}

/**
 * Reads the directives a section value holds for one scope.
 *
 * @param value - A section layer
 * @param scope - {@link CSP_DEFAULT_SCOPE} or a path location
 * @returns That scope's directives, if any
 *
 * @public
 */
export function scopeDirectives(value: CspValue | undefined, scope: string): CspDirectives | undefined {
  if (scope === CSP_DEFAULT_SCOPE) {
    return value?.directives;
  }
  return value?.paths?.find((path) => path.location === scope)?.directives;
}

/**
 * Lists the editable scopes: the top-level directives, then each path location in either layer.
 *
 * @param data - The fetched CSP config
 * @returns The scope keys, in display order
 *
 * @public
 */
export function cspScopes(data: CspConfigResponse | undefined): string[] {
  const locations = [...(data?.readOnly.paths ?? []), ...(data?.writable.paths ?? [])].map((path) => path.location);
  return [CSP_DEFAULT_SCOPE, ...new Set(locations)];
}

/**
 * Lists the sources a row sends: its override, or its default value.
 *
 * @param row - The row
 * @returns The row's sources
 *
 * @public
 */
export function rowSources(row: CspScopeRow): string[] {
  return row.override ?? row.base;
}

function toRows(base: Map<string, string[]>, draftRows: CspDirectiveDraftRow[]): CspScopeRow[] {
  const toRow = (directive: string, sources: string[], row?: CspDirectiveDraftRow): CspScopeRow => ({
    directive,
    base: sources,
    override: row?.sources,
    draftId: row?.id,
    saved: row?.saved,
  });
  const rows = [...base].map(([directive, sources]) =>
    toRow(
      directive,
      sources,
      draftRows.find((row) => row.directive === directive),
    ),
  );
  draftRows.filter((row) => !base.has(row.directive)).forEach((row) => rows.push(toRow(row.directive, [], row)));
  return rows;
}

function hasDirectives(readOnly: CspValue | undefined, draft: CspDraftScopes, location: string): boolean {
  return (
    Object.keys(scopeDirectives(readOnly, location) ?? {}).length > 0 ||
    (draft[location] ?? []).some((row) => row.sources.length > 0)
  );
}

/**
 * Finds the path the server applies for a scope. A path left with no directives is dropped, so the
 * server falls back to the longest remaining location that is a prefix of it.
 */
function effectiveLocation(readOnly: CspValue | undefined, draft: CspDraftScopes, scope: string): string | undefined {
  if (hasDirectives(readOnly, draft, scope)) {
    return scope;
  }
  const locations = [...(readOnly?.paths ?? []).map((path) => path.location), ...Object.keys(draft)];
  return locations
    .filter(
      (location) =>
        location !== CSP_DEFAULT_SCOPE &&
        location !== scope &&
        scope.startsWith(location) &&
        hasDirectives(readOnly, draft, location),
    )
    .sort((a, b) => b.length - a.length)[0];
}

/**
 * Builds the rows shown for a scope. Shared starts from the baseline and the read-only top-level
 * directives; a path starts from the Shared result and its read-only path directives. Each layer
 * replaces a directive whole, as the backend does.
 *
 * @param readOnly - The read-only layer
 * @param draft - The directives set per scope
 * @param scope - {@link CSP_DEFAULT_SCOPE} or a path location
 * @returns One row per directive that applies to the scope
 *
 * @public
 */
export function buildScopeRows(readOnly: CspValue | undefined, draft: CspDraftScopes, scope: string): CspScopeRow[] {
  const allBase = new Map<string, string[]>(CSP_BASELINE.map(([directive, sources]) => [directive, [...sources]]));
  Object.entries(readOnly?.directives ?? {}).forEach(([directive, sources]) => allBase.set(directive, sources));
  const allRows = toRows(allBase, draft[CSP_DEFAULT_SCOPE] ?? []);
  if (scope === CSP_DEFAULT_SCOPE) {
    return allRows;
  }

  const pathBase = new Map<string, string[]>(allRows.map((row) => [row.directive, rowSources(row)]));
  const location = effectiveLocation(readOnly, draft, scope);
  if (location !== undefined) {
    Object.entries(scopeDirectives(readOnly, location) ?? {}).forEach(([directive, sources]) =>
      pathBase.set(directive, sources),
    );
    if (location !== scope) {
      (draft[location] ?? [])
        .filter((row) => row.sources.length > 0)
        .forEach((row) => pathBase.set(row.directive, row.sources));
    }
  }
  return toRows(new Map([...pathBase].filter(([, sources]) => sources.length > 0)), draft[scope] ?? []);
}

/**
 * Builds the draft from the saved writable layer.
 *
 * @param data - The fetched CSP config
 * @returns The directives set per scope
 *
 * @public
 */
export function toCspDraft(data: CspConfigResponse | undefined): CspDraftScopes {
  return Object.fromEntries(
    cspScopes(data).map((scope) => [
      scope,
      Object.entries(scopeDirectives(data?.writable, scope) ?? {}).map(([directive, sources]) => ({
        ...createCspRow(directive, [...sources]),
        saved: true,
      })),
    ]),
  );
}

function toDirectives(rows: CspDirectiveDraftRow[]): CspDirectives | undefined {
  const entries = rows
    .filter((row) => row.sources.length > 0)
    .map((row): [string, string[]] => [row.directive, row.sources]);
  return entries.length > 0 ? Object.fromEntries(entries) : undefined;
}

/**
 * Builds the PUT body. Fields the UI does not edit, such as `reportUri`, are kept.
 *
 * @param writable - The saved writable layer
 * @param draft - The directives set per scope
 * @param reportOnly - The report-only choice, or `undefined` to keep the saved writable value
 * @returns The writable value to persist
 *
 * @public
 */
export function buildCspPayload(
  writable: CspValue | undefined,
  draft: CspDraftScopes,
  reportOnly: boolean | undefined,
): CspValue {
  const payload: CspValue = {...writable};
  const resolvedReportOnly = reportOnly ?? writable?.reportOnly;
  if (resolvedReportOnly === undefined) {
    delete payload.reportOnly;
  } else {
    payload.reportOnly = resolvedReportOnly;
  }

  const directives = toDirectives(draft[CSP_DEFAULT_SCOPE] ?? []);
  if (directives) {
    payload.directives = directives;
  } else {
    delete payload.directives;
  }

  const paths = Object.entries(draft)
    .filter(([scope]) => scope !== CSP_DEFAULT_SCOPE)
    .map(([location, rows]) => ({location, directives: toDirectives(rows)}))
    .filter((path): path is {location: string; directives: CspDirectives} => path.directives !== undefined);
  if (paths.length > 0) {
    payload.paths = paths;
  } else {
    delete payload.paths;
  }

  return payload;
}
