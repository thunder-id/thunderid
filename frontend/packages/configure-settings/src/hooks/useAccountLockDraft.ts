// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useCallback, useMemo, useState} from 'react';
import {useTranslation} from 'react-i18next';
import {
  AccountLockCategories,
  type AccountAccessConfigResponse,
  type AccountAccessValue,
  type AccountLockCategory,
} from '../models/accountAccess';
import {
  buildAccountLockPayload,
  isSameAccountLockForm,
  toAccountLockForm,
  type AccountLockForm,
} from '../utils/accountLockPayload';

/**
 * Blocking messages for one category, keyed by the field they belong to.
 *
 * @public
 */
export type AccountLockErrors = Partial<Record<keyof AccountLockForm, string>>;

/**
 * The editable draft of the Account security tab returned by {@link useAccountLockDraft}: the lock
 * policy of each category, under one save.
 *
 * @public
 */
export interface AccountLockDraft {
  /** The current values, by category. */
  forms: Record<AccountLockCategory, AccountLockForm>;
  /** Blocking messages, by category. */
  errors: Record<AccountLockCategory, AccountLockErrors>;
  /** Whether the draft differs from the saved effective value. */
  dirty: boolean;
  /** Whether any field currently has a blocking message. */
  hasErrors: boolean;
  /** Applies a partial change to one category. */
  change: (category: AccountLockCategory, patch: Partial<AccountLockForm>) => void;
  /** Discards local edits. */
  reset: () => void;
  /** Validates every category, shows the messages, and reports whether the draft is savable. */
  validateAll: () => boolean;
  /** Builds the writable document to send. */
  buildPayload: () => AccountAccessValue;
}

/**
 * Manages the `accountAccess` draft as a local overlay over the server value, so a background refetch
 * does not clobber edits in progress.
 *
 * The draft is seeded from the **effective** value rather than from the writable layer: the operator
 * is editing the policy that is in force, not the subset of it this deployment has previously
 * changed. What gets written back is only what they changed, which is
 * {@link buildAccountLockPayload}'s job and the reason the loaded values are kept beside the edited
 * ones.
 *
 * @param data - The fetched section, or `undefined` while loading
 * @returns The draft state and operations
 *
 * @public
 */
export default function useAccountLockDraft(data: AccountAccessConfigResponse | undefined): AccountLockDraft {
  const {t} = useTranslation();
  const [edited, setEdited] = useState<Record<AccountLockCategory, AccountLockForm> | undefined>(undefined);
  const [errors, setErrors] = useState<Record<AccountLockCategory, AccountLockErrors>>({user: {}, agent: {}});

  // Keyed on the serialized value rather than on `data`, so a refetch returning an equal value does
  // not rebuild the forms under the operator's cursor.
  const mergedKey: string = JSON.stringify(data?.merged ?? {});
  const saved = useMemo<Record<AccountLockCategory, AccountLockForm>>(() => {
    const merged = JSON.parse(mergedKey) as AccountAccessValue;
    return AccountLockCategories.reduce(
      (acc, category) => ({...acc, [category]: toAccountLockForm(merged, category)}),
      {} as Record<AccountLockCategory, AccountLockForm>,
    );
  }, [mergedKey]);

  const forms: Record<AccountLockCategory, AccountLockForm> = edited ?? saved;

  const dirty: boolean = AccountLockCategories.some(
    (category) => !isSameAccountLockForm(forms[category], saved[category]),
  );

  const validate = useCallback(
    (values: Record<AccountLockCategory, AccountLockForm>): Record<AccountLockCategory, AccountLockErrors> =>
      AccountLockCategories.reduce(
        (acc, category) => {
          const form: AccountLockForm = values[category];
          const found: AccountLockErrors = {};
          // A switched-off category holds no policy anyone has to defend, so its numbers are not
          // checked. Switching it back on re-checks them.
          if (form.enabled) {
            if (!Number.isInteger(form.threshold) || form.threshold < 1) {
              found.threshold = t(
                'settings:accountLock.validation.threshold',
                'Enter how many failed attempts should lock the account.',
              );
            }
            if (form.lockDurationsSeconds.length === 0) {
              found.lockDurationsSeconds = t(
                'settings:accountLock.validation.ladder',
                'Add at least one lock duration.',
              );
            }
            // Below the threshold the first lock would already be permanent, which the server refuses.
            const ceiling: number = form.maxConsecutiveFailures;
            if (!Number.isInteger(ceiling) || ceiling < 0 || (ceiling > 0 && ceiling < form.threshold)) {
              found.maxConsecutiveFailures = t(
                'settings:accountLock.validation.ceiling',
                'Enter 0 for no limit, or at least the number of failed attempts that locks the account.',
              );
            }
          }
          return {...acc, [category]: found};
        },
        {} as Record<AccountLockCategory, AccountLockErrors>,
      ),
    [t],
  );

  const hasErrors: boolean = AccountLockCategories.some((category) => Object.keys(errors[category]).length > 0);

  const change = useCallback(
    (category: AccountLockCategory, patch: Partial<AccountLockForm>): void => {
      const next: Record<AccountLockCategory, AccountLockForm> = {
        ...forms,
        [category]: {...forms[category], ...patch},
      };
      setEdited(next);
      // Re-validated on every change: every control is a number, a choice or a switch.
      setErrors(validate(next));
    },
    [forms, validate],
  );

  const reset = useCallback((): void => {
    setEdited(undefined);
    setErrors({user: {}, agent: {}});
  }, []);

  const validateAll = useCallback((): boolean => {
    const found: Record<AccountLockCategory, AccountLockErrors> = validate(forms);
    setErrors(found);
    return AccountLockCategories.every((category) => Object.keys(found[category]).length === 0);
  }, [forms, validate]);

  const buildPayload = useCallback(
    (): AccountAccessValue => buildAccountLockPayload(data?.writable, saved, forms, data?.merged),
    [data?.writable, saved, forms, data?.merged],
  );

  return {forms, errors, dirty, hasErrors, change, reset, validateAll, buildPayload};
}
