// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {CspDraftScopes, CspScopeRow} from '../models/cspDirectiveRow';
import type {CspConfigResponse, CspValue} from '../models/responses';
import {
  buildCspPayload,
  buildScopeRows,
  createCspRow,
  CSP_DIRECTIVES,
  CSP_NONE_SOURCE,
  cspScopes,
  rowSources,
  toCspDraft,
} from '../utils/cspDirectiveRows';
import validateCspDirectiveRows, {CspDirectiveRowIssueFallbacks} from '../utils/validateCspDirectiveRows';

/**
 * The editable draft of the CSP section returned by {@link useCspDraft}.
 *
 * @public
 */
export interface CspDraft {
  /** The editable scopes: the top-level directives first, then each path location. */
  scopes: string[];
  /** The report-only choice shown to the admin. */
  reportOnly: boolean;
  /** Error messages, keyed by draft row id. */
  errors: Record<string, string>;
  /** Whether the draft differs from the saved value. */
  dirty: boolean;
  /** Whether any directive has a blocking error. */
  hasErrors: boolean;
  /** Lists the directives that apply to a scope. */
  rowsFor: (scope: string) => CspScopeRow[];
  /** Lists the directives a scope does not have yet. */
  availableDirectives: (scope: string) => string[];
  setReportOnly: (reportOnly: boolean) => void;
  /** Sets a directive's sources in a scope, overriding its default value. */
  setSources: (scope: string, directive: string, sources: string[]) => void;
  /** Appends a directive that the scope does not have yet. */
  addRow: (scope: string) => void;
  changeDirective: (scope: string, draftId: string, directive: string) => void;
  /** Drops a directive the admin set, which resets it to its default value. */
  removeRow: (scope: string, draftId: string) => void;
  /** Clears local edits, reverting to the saved server value (used by Reset and after a save). */
  reset: () => void;
  /** Builds the PUT body. */
  buildPayload: () => CspValue;
}

/**
 * Manages the editable writable layer of the CSP section as a local overlay over the server value, so a
 * background refetch does not clobber in-progress edits.
 *
 * @param data - The fetched CSP config, or `undefined` while loading
 * @returns The draft state and operations
 *
 * @public
 */
export default function useCspDraft(data: CspConfigResponse | undefined): CspDraft {
  const {t} = useTranslation();
  const [editedDraft, setEditedDraft] = useState<CspDraftScopes | undefined>(undefined);
  const [editedReportOnly, setEditedReportOnly] = useState<boolean | undefined>(undefined);

  // Keyed on the serialized value rather than on `data`: building rows mints new ids, so a refetch
  // returning an equal value would otherwise remount every untouched field.
  const dataKey = JSON.stringify(data ?? null);
  const scopes = useMemo(
    () => cspScopes(data),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [dataKey],
  );
  const savedDraft = useMemo(
    () => toCspDraft(data),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [dataKey],
  );

  const draft = editedDraft ?? savedDraft;
  // An unset value means report-only on the server, so the switch shows the same.
  const savedReportOnly = data?.merged.reportOnly ?? true;
  const reportOnly = editedReportOnly ?? savedReportOnly;

  const rowsFor = useCallback(
    (scope: string): CspScopeRow[] => buildScopeRows(data?.readOnly, draft, scope),
    [data, draft],
  );

  const errors = useMemo<Record<string, string>>(
    () =>
      Object.fromEntries(
        Object.entries(validateCspDirectiveRows(scopes.flatMap((scope) => rowsFor(scope)))).map(([id, code]) => [
          id,
          t(`settings:csp.validation.${code}`, CspDirectiveRowIssueFallbacks[code]),
        ]),
      ),
    [scopes, rowsFor, t],
  );

  const availableDirectives = useCallback(
    (scope: string): string[] => {
      const used = new Set(rowsFor(scope).map((row) => row.directive));
      return CSP_DIRECTIVES.filter((directive) => !used.has(directive));
    },
    [rowsFor],
  );

  const setSources = useCallback(
    (scope: string, directive: string, sources: string[]): void => {
      const current = rowsFor(scope).find((row) => row.directive === directive);
      // Adding a source to a directive that allows nothing replaces 'none'.
      const next =
        current && rowSources(current).includes(CSP_NONE_SOURCE) && sources.length > 1
          ? sources.filter((source) => source !== CSP_NONE_SOURCE)
          : sources;
      const rows = draft[scope] ?? [];
      const exists = rows.some((row) => row.directive === directive);
      setEditedDraft({
        ...draft,
        [scope]: exists
          ? rows.map((row) => (row.directive === directive ? {...row, sources: next} : row))
          : [...rows, createCspRow(directive, next)],
      });
    },
    [draft, rowsFor],
  );

  const addRow = useCallback(
    (scope: string): void => {
      const [directive] = availableDirectives(scope);
      if (directive) {
        setEditedDraft({...draft, [scope]: [...(draft[scope] ?? []), createCspRow(directive)]});
      }
    },
    [draft, availableDirectives],
  );

  const changeDirective = useCallback(
    (scope: string, draftId: string, directive: string): void =>
      setEditedDraft({
        ...draft,
        [scope]: (draft[scope] ?? []).map((row) => (row.id === draftId ? {...row, directive} : row)),
      }),
    [draft],
  );

  const removeRow = useCallback(
    (scope: string, draftId: string): void =>
      setEditedDraft({...draft, [scope]: (draft[scope] ?? []).filter((row) => row.id !== draftId)}),
    [draft],
  );

  const reset = useCallback((): void => {
    setEditedDraft(undefined);
    setEditedReportOnly(undefined);
  }, []);

  const buildPayload = useCallback(
    (): CspValue => buildCspPayload(data?.writable, draft, editedReportOnly),
    [data, draft, editedReportOnly],
  );

  const dirty = useMemo<boolean>(() => {
    // An empty new directive is not a change, but emptying an existing one is.
    const key = (scoped: CspDraftScopes): string =>
      JSON.stringify(
        scopes.map((scope) =>
          buildScopeRows(data?.readOnly, scoped, scope)
            .filter(
              (row) => row.override !== undefined && (row.override.length > 0 || row.base.length > 0 || row.saved),
            )
            .map((row) => [row.directive, row.override]),
        ),
      );
    return reportOnly !== savedReportOnly || key(draft) !== key(savedDraft);
  }, [scopes, data, draft, savedDraft, reportOnly, savedReportOnly]);

  return {
    scopes,
    reportOnly,
    errors,
    dirty,
    hasErrors: Object.keys(errors).length > 0,
    rowsFor,
    availableDirectives,
    setReportOnly: setEditedReportOnly,
    setSources,
    addRow,
    changeDirective,
    removeRow,
    reset,
    buildPayload,
  };
}
