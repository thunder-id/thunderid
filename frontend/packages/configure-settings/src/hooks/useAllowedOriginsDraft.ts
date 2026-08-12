// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {AllowedOriginTypes, type AllowedOriginDraftRow, type AllowedOriginType} from '../models/allowedOriginRow';
import type {CorsConfigResponse, CorsValue} from '../models/responses';
import {createRow, normalizeRowValue, rowKey, toAllowedOrigins, toRows} from '../utils/allowedOriginRows';
import baselineKey from '../utils/baselineKey';
import validateAllowedOriginRows, {
  AllowedOriginRowIssueFallbacks,
  type AllowedOriginRowError,
  type AllowedOriginRowWarning,
} from '../utils/validateAllowedOriginRows';

/**
 * The editable draft of writable CORS origins returned by {@link useAllowedOriginsDraft}.
 *
 * @public
 */
export interface AllowedOriginsDraft {
  /** Editable rows, each carrying the type the admin chose. */
  draft: AllowedOriginDraftRow[];
  /** Blocking validation and duplicate messages, keyed by row id. */
  errors: Record<string, string>;
  /** Non-blocking cautions, keyed by row id. */
  warnings: Record<string, string>;
  /** Whether the draft differs from the saved baseline. */
  dirty: boolean;
  /** Whether any row currently has a blocking error. */
  hasErrors: boolean;
  /** Appends an empty literal-origin row. */
  addRow: () => void;
  /** Removes the row with the given id and re-validates the remaining rows. */
  removeRow: (id: string) => void;
  /** Updates a row's value; its own messages stay hidden until blur. */
  changeRow: (id: string, value: string) => void;
  /** Switches a row between a literal origin and a regex, keeping the text it already holds. */
  changeRowType: (id: string, type: AllowedOriginType) => void;
  /** Normalizes and validates the row with the given id. */
  blurRow: (id: string) => void;
  /** Clears local edits, reverting to the saved server value (used by Reset and after a save). */
  reset: () => void;
  /** Validates every row, sets messages, and returns whether the draft is savable. */
  validateAll: () => boolean;
  /** Builds the PUT body from each row's declared type. */
  buildPayload: () => CorsValue;
}

/**
 * Manages the editable draft of writable CORS origins as a local overlay over the server value, so a
 * background refetch does not clobber in-progress edits. Each row's type comes from the saved entry's
 * shape and is then the admin's to change, so nothing is ever reclassified from its text.
 *
 * @param data - The fetched CORS config, or `undefined` while loading
 * @returns The draft state and operations: add/remove/edit, validation, dirty tracking, and payload building
 *
 * @public
 */
export default function useAllowedOriginsDraft(data: CorsConfigResponse | undefined): AllowedOriginsDraft {
  const {t} = useTranslation();
  const [editedDraft, setEditedDraft] = useState<AllowedOriginDraftRow[] | undefined>(undefined);
  const [errors, setErrors] = useState<Record<string, string>>({});
  const [warnings, setWarnings] = useState<Record<string, string>>({});

  // Both memos key on the serialized entries rather than on `data`: building rows mints new ids, so
  // a refetch returning an equal value would otherwise remount every untouched field.
  const savedEntriesKey = JSON.stringify(data?.writable.allowedOrigins ?? []);
  const readOnlyEntriesKey = JSON.stringify(data?.readOnly.allowedOrigins ?? []);

  const savedRows = useMemo<AllowedOriginDraftRow[]>(
    () => toRows(data?.writable.allowedOrigins ?? []),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [savedEntriesKey],
  );
  const readOnlyKeys = useMemo<Set<string>>(
    () => new Set(toRows(data?.readOnly.allowedOrigins ?? []).map(rowKey)),
    // eslint-disable-next-line react-hooks/exhaustive-deps
    [readOnlyEntriesKey],
  );

  const draft = editedDraft ?? savedRows;

  /**
   * Validates rows and resolves each issue code to a localized message. Codes are resolved
   * dynamically, so every lookup carries the code's own default copy as its fallback.
   */
  const computeIssues = useCallback(
    (rows: AllowedOriginDraftRow[]): {errors: Record<string, string>; warnings: Record<string, string>} => {
      const issues = validateAllowedOriginRows(rows, readOnlyKeys);
      const resolve = (
        codes: Record<string, AllowedOriginRowError | AllowedOriginRowWarning>,
      ): Record<string, string> =>
        Object.fromEntries(
          Object.entries(codes).map(([id, code]) => [
            id,
            t(`settings:cors.validation.${code}`, AllowedOriginRowIssueFallbacks[code]),
          ]),
        );
      return {errors: resolve(issues.errors), warnings: resolve(issues.warnings)};
    },
    [readOnlyKeys, t],
  );

  /**
   * Publishes the messages for a set of rows, optionally keeping one row quiet. The row being typed
   * into is silenced until it is blurred, so a half-typed origin is not reported as invalid.
   */
  const applyIssues = useCallback(
    (rows: AllowedOriginDraftRow[], quietRowId?: string): void => {
      const issues = computeIssues(rows);
      if (quietRowId !== undefined) {
        delete issues.errors[quietRowId];
        delete issues.warnings[quietRowId];
      }
      setErrors(issues.errors);
      setWarnings(issues.warnings);
    },
    [computeIssues],
  );

  const addRow = useCallback((): void => {
    setEditedDraft([...draft, createRow(AllowedOriginTypes.ORIGIN)]);
  }, [draft]);

  const removeRow = useCallback(
    (id: string): void => {
      const next = draft.filter((row) => row.id !== id);
      setEditedDraft(next);
      // Removing a row can clear duplicate errors on the remaining rows.
      applyIssues(next);
    },
    [draft, applyIssues],
  );

  const changeRow = useCallback(
    (id: string, value: string): void => {
      const next = draft.map((row) => (row.id === id ? {...row, value} : row));
      setEditedDraft(next);
      // Keep the active row quiet until blur, while clearing stale messages on other rows.
      applyIssues(next, id);
    },
    [draft, applyIssues],
  );

  const changeRowType = useCallback(
    (id: string, type: AllowedOriginType): void => {
      // Switching type is a deliberate, discrete action rather than mid-typing, so the text carries
      // over untouched, is re-canonicalized for the new type, and is validated straight away.
      const next = draft.map((row) => {
        if (row.id !== id) {
          return row;
        }
        const retyped = {...row, type};
        return {...retyped, value: normalizeRowValue(retyped)};
      });
      setEditedDraft(next);
      applyIssues(next);
    },
    [draft, applyIssues],
  );

  const blurRow = useCallback(
    (id: string): void => {
      const next = draft.map((row) => (row.id === id ? {...row, value: normalizeRowValue(row)} : row));
      setEditedDraft(next);
      applyIssues(next);
    },
    [draft, applyIssues],
  );

  const reset = useCallback((): void => {
    setEditedDraft(undefined);
    setErrors({});
    setWarnings({});
  }, []);

  const validateAll = useCallback((): boolean => {
    const issues = computeIssues(draft);
    setErrors(issues.errors);
    setWarnings(issues.warnings);
    return Object.keys(issues.errors).length === 0;
  }, [draft, computeIssues]);

  const buildPayload = useCallback((): CorsValue => ({allowedOrigins: toAllowedOrigins(draft)}), [draft]);

  const dirty = useMemo<boolean>(() => baselineKey(draft) !== baselineKey(savedRows), [draft, savedRows]);
  const hasErrors: boolean = Object.keys(errors).length > 0;

  return {
    draft,
    errors,
    warnings,
    dirty,
    hasErrors,
    addRow,
    removeRow,
    changeRow,
    changeRowType,
    blurRow,
    reset,
    validateAll,
    buildPayload,
  };
}
