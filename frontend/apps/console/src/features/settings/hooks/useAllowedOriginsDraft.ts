/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import type {AllowedOrigin, CorsConfigResponse, CorsValue} from '../models/responses';
import baselineKey from '../utils/baselineKey';
import {isValidOrigin, isValidRegex, normalizeOrigin} from '../utils/origin';
import originValueText from '../utils/originValueText';

/**
 * The editable draft of writable CORS origins returned by {@link useAllowedOriginsDraft}.
 *
 * @public
 */
export interface AllowedOriginsDraft {
  /** Editable rows. Each is a literal origin (incl. `"null"`) or a regex pattern, kept as text. */
  draft: string[];
  /** Per-row validation/duplicate error messages, keyed by draft index. */
  errors: Record<number, string>;
  /** Whether the normalized draft differs from the saved baseline. */
  dirty: boolean;
  /** Whether any row currently has a validation/duplicate error. */
  hasErrors: boolean;
  /** Appends an empty editable row. */
  addRow: () => void;
  /** Removes the row at the given index and re-validates the remaining rows. */
  removeRow: (index: number) => void;
  /** Updates the row at the given index; its own error stays hidden until blur. */
  changeRow: (index: number, value: string) => void;
  /** Normalizes and validates the row at the given index. */
  blurRow: (index: number) => void;
  /** Clears local edits, reverting to the saved server value (used by Reset and after a save). */
  reset: () => void;
  /** Validates every row, sets errors, and returns whether the draft is savable. */
  validateAll: () => boolean;
  /** Builds the PUT body, classifying each row as a literal origin or a `{regex}` entry. */
  buildPayload: () => CorsValue;
}

/**
 * Manages the editable draft of writable CORS origins as a local overlay over the server value, so a
 * background refetch does not clobber in-progress edits. On save, each row is classified as a literal
 * origin or a `{regex}` entry.
 *
 * @param data - The fetched CORS config, or `undefined` while loading
 * @returns The draft state and operations: add/remove/edit, validation, dirty tracking, and payload building
 *
 * @public
 */
export default function useAllowedOriginsDraft(data: CorsConfigResponse | undefined): AllowedOriginsDraft {
  const {t} = useTranslation();
  const [editedDraft, setEditedDraft] = useState<string[] | undefined>(undefined);
  const [errors, setErrors] = useState<Record<number, string>>({});

  const savedValues = useMemo<string[]>(() => (data?.writable.allowedOrigins ?? []).map(originValueText), [data]);
  const readOnlyNormalized = useMemo<string[]>(
    () => (data?.readOnly.allowedOrigins ?? []).map(originValueText).map(normalizeOrigin),
    [data],
  );

  const draft = editedDraft ?? savedValues;

  const computeErrors = useCallback(
    (rows: string[]): Record<number, string> => {
      const normalized = rows.map(normalizeOrigin);
      const counts = new Map<string, number>();
      normalized.forEach((value) => {
        if (value !== '') {
          counts.set(value, (counts.get(value) ?? 0) + 1);
        }
      });
      const readOnlySet = new Set(readOnlyNormalized);
      const nextErrors: Record<number, string> = {};
      normalized.forEach((value, index) => {
        if (value === '') {
          return;
        }
        if (!isValidOrigin(value) && !isValidRegex(value)) {
          nextErrors[index] = t('settings:cors.validation.invalid');
        } else if ((counts.get(value) ?? 0) > 1 || readOnlySet.has(value)) {
          nextErrors[index] = t('settings:cors.validation.duplicate');
        }
      });
      return nextErrors;
    },
    [readOnlyNormalized, t],
  );

  const addRow = useCallback((): void => {
    setEditedDraft([...draft, '']);
  }, [draft]);

  const removeRow = useCallback(
    (index: number): void => {
      const next = draft.filter((_, i) => i !== index);
      setEditedDraft(next);
      // Removing a row can clear duplicate errors on the remaining rows.
      setErrors(computeErrors(next));
    },
    [draft, computeErrors],
  );

  const changeRow = useCallback(
    (index: number, value: string): void => {
      const next = [...draft];
      next[index] = value;
      setEditedDraft(next);
      // Keep the active row quiet until blur, while clearing stale errors on other rows.
      const recomputed = computeErrors(next);
      delete recomputed[index];
      setErrors(recomputed);
    },
    [draft, computeErrors],
  );

  const blurRow = useCallback(
    (index: number): void => {
      const next = [...draft];
      next[index] = normalizeOrigin(next[index] ?? '');
      setEditedDraft(next);
      setErrors(computeErrors(next));
    },
    [draft, computeErrors],
  );

  const reset = useCallback((): void => {
    setEditedDraft(undefined);
    setErrors({});
  }, []);

  const validateAll = useCallback((): boolean => {
    const nextErrors = computeErrors(draft);
    setErrors(nextErrors);
    return Object.keys(nextErrors).length === 0;
  }, [draft, computeErrors]);

  const buildPayload = useCallback((): CorsValue => {
    const entries: AllowedOrigin[] = [];
    draft.forEach((raw) => {
      const value = normalizeOrigin(raw);
      if (value === '') {
        return;
      }
      entries.push(isValidOrigin(value) ? value : {regex: value});
    });
    return {allowedOrigins: entries};
  }, [draft]);

  const dirty = useMemo<boolean>(() => baselineKey(draft) !== baselineKey(savedValues), [draft, savedValues]);
  const hasErrors: boolean = Object.keys(errors).length > 0;

  return {
    draft,
    errors,
    dirty,
    hasErrors,
    addRow,
    removeRow,
    changeRow,
    blurRow,
    reset,
    validateAll,
    buildPayload,
  };
}
