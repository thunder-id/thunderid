// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {isEqual} from '@thunderid/utils';
import type {TFunction} from 'i18next';
import {useCallback, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {
  SessionTimeoutFields,
  type SessionConfigResponse,
  type SessionTimeoutField,
  type SessionValue,
} from '../models/session';

/** The draft's change to each field: a number sets it, and `null` removes it so the default applies. */
type SessionTimeoutEdits = Partial<Record<SessionTimeoutField, number | null>>;

/**
 * Blocking messages, keyed by the field they belong to.
 *
 * @public
 */
export type SessionTimeoutErrors = Partial<Record<SessionTimeoutField, string>>;

/**
 * The editable draft of the session timeouts returned by {@link useSessionTimeoutsDraft}.
 *
 * @public
 */
export interface SessionTimeoutsDraft {
  /**
   * The value each field shows: the draft's, else the value in effect. A field reset to its default
   * shows `0`, since the default is known only once the save returns the value in effect.
   */
  values: Record<SessionTimeoutField, number>;
  /** Whether the section sets each field (Custom) rather than leaving it to the default. */
  custom: Record<SessionTimeoutField, boolean>;
  /** Blocking messages, keyed by field. */
  errors: SessionTimeoutErrors;
  /** Whether saving the draft would change the section. */
  dirty: boolean;
  /** Whether any field has a blocking message. */
  hasErrors: boolean;
  /** Sets a field. */
  change: (field: SessionTimeoutField, seconds: number) => void;
  /** Removes a field from the section, so its default applies. */
  resetToDefault: (field: SessionTimeoutField) => void;
  /** Discards the draft. */
  reset: () => void;
  /** Builds the writable value to send. */
  buildPayload: () => SessionValue;
}

/**
 * Applies the draft's changes to the saved writable value. Every field the draft does not touch keeps
 * its saved value, and a field reset to its default is left out.
 */
function applyEdits(writable: SessionValue, edits: SessionTimeoutEdits): SessionValue {
  const next: SessionValue = {...writable};
  SessionTimeoutFields.forEach((field) => {
    const edit: number | null | undefined = edits[field];
    if (edit === null) {
      delete next[field];
    } else if (edit !== undefined) {
      next[field] = edit;
    }
  });
  return next;
}

/**
 * Checks the draft as the server does: each value set here is a whole number above zero, and the idle
 * timeout is compared with the absolute one only when both are set here, since an absolute timeout set
 * on its own also cuts the idle timeout in effect. A message that involves both fields goes on the one
 * changed last, where the operator is looking.
 */
function validate(payload: SessionValue, changed: SessionTimeoutField | undefined, t: TFunction): SessionTimeoutErrors {
  const errors: SessionTimeoutErrors = {};
  SessionTimeoutFields.forEach((field) => {
    const value: number | undefined = payload[field];
    if (value !== undefined && (!Number.isInteger(value) || value <= 0)) {
      errors[field] = t('settings:session.validation.wholeNumber', 'Enter a whole number greater than zero.');
    }
  });
  const {idleTimeoutSeconds: idle, absoluteTimeoutSeconds: absolute} = payload;
  if (Object.keys(errors).length === 0 && idle !== undefined && absolute !== undefined && idle > absolute) {
    errors[changed ?? 'idleTimeoutSeconds'] = t(
      'settings:session.validation.idleExceedsAbsolute',
      "The idle timeout can't be longer than the absolute timeout.",
    );
  }
  return errors;
}

/**
 * Whether a declarative resource sets either timeout. The server refuses every write to the section
 * then, so it can't be changed here.
 *
 * @param data - The fetched section, or `undefined` while loading
 * @returns Whether the section is set declaratively
 *
 * @public
 */
export function isSessionDeclarative(data: SessionConfigResponse | undefined): boolean {
  return SessionTimeoutFields.some((field) => data?.readOnly?.[field] !== undefined);
}

/**
 * Manages the `session` draft as a local overlay over the server value, so a background refetch does
 * not clobber edits in progress.
 *
 * Each field shows the value in effect until it is changed. A save sends the saved writable value
 * with the draft's changes applied: a field set here stays set, and a field reset to its default is
 * left out, which is how the server returns it to the default.
 *
 * A later read that finds a declarative resource setting a timeout ends the draft, since the server
 * refuses every write to the section then.
 *
 * @param data - The fetched section, or `undefined` while loading
 * @returns The draft state and operations
 *
 * @public
 */
export default function useSessionTimeoutsDraft(data: SessionConfigResponse | undefined): SessionTimeoutsDraft {
  const {t} = useTranslation();
  const [edits, setEdits] = useState<SessionTimeoutEdits>({});
  const [lastChanged, setLastChanged] = useState<SessionTimeoutField | undefined>(undefined);

  // A declarative resource that arrives with a later read ends the draft: it could never be saved.
  const declarative: boolean = isSessionDeclarative(data);
  const [wasDeclarative, setWasDeclarative] = useState<boolean>(declarative);
  if (declarative !== wasDeclarative) {
    setWasDeclarative(declarative);
    if (declarative) {
      setEdits({});
      setLastChanged(undefined);
    }
  }

  const writable: SessionValue = data?.writable ?? {};
  const draftWritable: SessionValue = applyEdits(writable, edits);
  // While a declarative resource sets the section, it takes the writable layer's place.
  const configured: SessionValue = declarative && data ? data.readOnly : draftWritable;

  const valueOf = (field: SessionTimeoutField): number => {
    const edit: number | null | undefined = edits[field];
    if (edit === null) {
      return 0;
    }
    return edit ?? data?.merged[field] ?? 0;
  };
  const values: Record<SessionTimeoutField, number> = {
    idleTimeoutSeconds: valueOf('idleTimeoutSeconds'),
    absoluteTimeoutSeconds: valueOf('absoluteTimeoutSeconds'),
  };
  const custom: Record<SessionTimeoutField, boolean> = {
    idleTimeoutSeconds: configured.idleTimeoutSeconds !== undefined,
    absoluteTimeoutSeconds: configured.absoluteTimeoutSeconds !== undefined,
  };

  const dirty = !isEqual(draftWritable, writable);
  // Nothing is checked before the first change, so the card never opens on a complaint about values
  // already in effect.
  const errors: SessionTimeoutErrors = dirty ? validate(draftWritable, lastChanged, t) : {};

  const edit = useCallback(
    (field: SessionTimeoutField, value: number | null): void => {
      // An edit back to the saved value is no edit, so a later read shows that read's value rather
      // than an unsaved one.
      const saved: number | undefined = data?.writable?.[field];
      const reverted: boolean = value === null ? saved === undefined : value === saved;
      const next: SessionTimeoutEdits = {...edits, [field]: value};
      if (reverted) {
        delete next[field];
      }
      setEdits(next);
      setLastChanged(field);
    },
    [edits, data?.writable],
  );

  const change = useCallback((field: SessionTimeoutField, seconds: number): void => edit(field, seconds), [edit]);

  const resetToDefault = useCallback((field: SessionTimeoutField): void => edit(field, null), [edit]);

  const reset = useCallback((): void => {
    setEdits({});
    setLastChanged(undefined);
  }, []);

  const buildPayload = useCallback(
    (): SessionValue => applyEdits(data?.writable ?? {}, edits),
    [data?.writable, edits],
  );

  return {
    values,
    custom,
    errors,
    dirty,
    hasErrors: Object.keys(errors).length > 0,
    change,
    resetToDefault,
    reset,
    buildPayload,
  };
}
