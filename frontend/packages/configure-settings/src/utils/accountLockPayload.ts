// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {
  AccountAccessCategory,
  AccountAccessLockEmail,
  AccountAccessPolicy,
  AccountAccessValue,
  AccountLockCategory,
  LockGranularity,
} from '../models/accountAccess';

/**
 * The fields the Account security tab edits for one category: the lock policy and sign-in activity.
 * Everything else in the section is preserved as it was saved.
 *
 * @public
 */
export interface AccountLockForm {
  enabled: boolean;
  granularity: LockGranularity;
  threshold: number;
  failureWindowSeconds: number;
  lockDurationsSeconds: number[];
  lockDecaySeconds: number;
  /** Whether a committed lock emails the account holder. Users only; an agent has no holder to tell. */
  notifyEnabled: boolean;
  notifyRecipientAttribute: string;
  /** Whether a successful sign-in records when it happened. */
  recordLastLogin: boolean;
  /** How stale the recorded sign-in time may get before a sign-in writes a new one. */
  lastLoginResolutionSeconds: number;
}

/**
 * The resolution the server uses when recording is on and none is set.
 *
 * @public
 */
export const DEFAULT_LAST_LOGIN_RESOLUTION_SECONDS = 3600;

/**
 * The effective policy for one category, as the form should first show it.
 *
 * @param merged - The section's effective value
 * @param category - The category to read
 * @returns The form values, with the product's own fallbacks where the section says nothing
 *
 * @public
 */
export function toAccountLockForm(
  merged: AccountAccessValue | undefined,
  category: AccountLockCategory,
): AccountLockForm {
  const policy: AccountAccessPolicy = merged?.[category]?.default ?? {};
  const email: AccountAccessLockEmail = merged?.[category]?.notifications?.onLock?.email ?? {};
  return {
    // As the server reads it: an unset switch is on when the policy can form a lock.
    enabled: policy.enabled ?? canFormLock(policy),
    granularity: merged?.[category]?.lockGranularity ?? 'authentication_method',
    threshold: policy.threshold ?? 0,
    failureWindowSeconds: policy.failureWindowSeconds ?? 0,
    lockDurationsSeconds: [...(policy.lockDurationsSeconds ?? [])],
    lockDecaySeconds: policy.lockDecaySeconds ?? 0,
    notifyEnabled: email.enabled ?? false,
    notifyRecipientAttribute: email.recipientAttribute ?? 'email',
    recordLastLogin: merged?.[category]?.recordLastLogin ?? false,
    lastLoginResolutionSeconds: merged?.[category]?.lastLoginResolutionSeconds ?? DEFAULT_LAST_LOGIN_RESOLUTION_SECONDS,
  };
}

/**
 * Whether a policy sets everything a lock needs: a threshold, a failure window and a duration.
 *
 * @param policy - The policy to check
 * @returns Whether it can form a lock
 */
function canFormLock(policy: AccountAccessPolicy): boolean {
  return (
    (policy.threshold ?? 0) > 0 &&
    policy.failureWindowSeconds !== undefined &&
    (policy.lockDurationsSeconds?.length ?? 0) > 0
  );
}

/**
 * Whether two forms hold the same values.
 *
 * @param a - One form
 * @param b - The other
 * @returns Whether they are equal
 *
 * @public
 */
export function isSameAccountLockForm(a: AccountLockForm, b: AccountLockForm): boolean {
  return (
    a.enabled === b.enabled &&
    a.granularity === b.granularity &&
    a.threshold === b.threshold &&
    a.failureWindowSeconds === b.failureWindowSeconds &&
    a.lockDecaySeconds === b.lockDecaySeconds &&
    a.notifyEnabled === b.notifyEnabled &&
    a.notifyRecipientAttribute === b.notifyRecipientAttribute &&
    a.recordLastLogin === b.recordLastLogin &&
    a.lastLoginResolutionSeconds === b.lastLoginResolutionSeconds &&
    a.lockDurationsSeconds.length === b.lockDurationsSeconds.length &&
    a.lockDurationsSeconds.every((value, index) => value === b.lockDurationsSeconds[index])
  );
}

/**
 * Builds the writable document to `PUT`, given the saved writable layer, the values as they were
 * loaded, and the edited values.
 *
 * Only what changed is written: a field still holding its loaded value keeps whatever the saved
 * layer holds. Writing everything the form displays would pin the deployment's current policy into
 * the runtime layer on the first edit, and dropping an untouched field would silently lose an
 * earlier override, since the `PUT` replaces the writable layer.
 *
 * Everything this screen does not edit is preserved: any field or category the Console does not
 * show is copied through. The lock cards and the sign-in activity card share this one document, so
 * a save from either keeps the other's edits.
 *
 * The baseline is the effective policy as loaded rather than the declarative layer, because that is
 * what the section overlays. One consequence: setting a field back to the value the deployment file
 * supplies writes it explicitly rather than removing the override.
 *
 * The failure window is the exception: when the effective policy has none, an enabled form writes
 * the window it shows, since the server cannot form a lock without one and zero is a valid choice.
 *
 * @param writable - The saved writable layer
 * @param baseline - The values as loaded, by category
 * @param forms - The edited values, by category
 * @param merged - The section's effective value as loaded
 * @returns The document to send
 *
 * @public
 */
export function buildAccountLockPayload(
  writable: AccountAccessValue | undefined,
  baseline: Record<AccountLockCategory, AccountLockForm>,
  forms: Record<AccountLockCategory, AccountLockForm>,
  merged: AccountAccessValue | undefined,
): AccountAccessValue {
  const next: AccountAccessValue = structuredClone(writable ?? {});

  (Object.keys(forms) as AccountLockCategory[]).forEach((category) => {
    const form: AccountLockForm = forms[category];
    const base: AccountLockForm = baseline[category];
    const saved: AccountAccessCategory = next[category] ?? {};

    const nextDefault: AccountAccessPolicy = {...(saved.default ?? {})};
    assign(nextDefault, 'enabled', form.enabled, base.enabled);
    assign(nextDefault, 'threshold', form.threshold, base.threshold);
    if (form.enabled && merged?.[category]?.default?.failureWindowSeconds === undefined) {
      nextDefault.failureWindowSeconds = form.failureWindowSeconds;
    } else {
      assign(nextDefault, 'failureWindowSeconds', form.failureWindowSeconds, base.failureWindowSeconds);
    }
    assign(nextDefault, 'lockDecaySeconds', form.lockDecaySeconds, base.lockDecaySeconds);
    assignLadder(nextDefault, form.lockDurationsSeconds, base.lockDurationsSeconds);

    const nextCategory: AccountAccessCategory = {...saved};
    if (form.granularity !== base.granularity) {
      nextCategory.lockGranularity = form.granularity;
    }
    if (form.recordLastLogin !== base.recordLastLogin) {
      nextCategory.recordLastLogin = form.recordLastLogin;
    }
    if (form.lastLoginResolutionSeconds !== base.lastLoginResolutionSeconds) {
      nextCategory.lastLoginResolutionSeconds = form.lastLoginResolutionSeconds;
    }

    if (Object.keys(nextDefault).length > 0) {
      nextCategory.default = nextDefault;
    } else {
      delete nextCategory.default;
    }

    assignNotifications(nextCategory, form, base);

    if (Object.keys(nextCategory).length > 0) {
      next[category] = nextCategory;
    } else {
      delete next[category];
    }
  });

  return next;
}

/** Writes changed email fields while preserving the rest of the saved block. */
function assignNotifications(target: AccountAccessCategory, form: AccountLockForm, base: AccountLockForm): void {
  const saved: AccountAccessLockEmail = target.notifications?.onLock?.email ?? {};
  const email: AccountAccessLockEmail = {...saved};

  if (form.notifyEnabled !== base.notifyEnabled) {
    email.enabled = form.notifyEnabled;
  }

  if (form.notifyRecipientAttribute !== base.notifyRecipientAttribute) {
    email.recipientAttribute = form.notifyRecipientAttribute;
  }

  if (Object.keys(email).length > 0) {
    target.notifications = {onLock: {email}};
    return;
  }
  delete target.notifications;
}

/**
 * Sets a field when it departs from the baseline, and otherwise keeps what the saved layer holds.
 */
function assign<K extends keyof AccountAccessPolicy>(
  target: AccountAccessPolicy,
  key: K,
  value: AccountAccessPolicy[K],
  baseline: AccountAccessPolicy[K],
): void {
  if (value === baseline) {
    return;
  }
  target[key] = value;
}

/**
 * The ladder replaces as a unit, so it is compared as one.
 */
function assignLadder(target: AccountAccessPolicy, value: number[], baseline: number[]): void {
  const same: boolean = baseline.length === value.length && value.every((entry, index) => entry === baseline[index]);
  if (same) {
    return;
  }
  target.lockDurationsSeconds = [...value];
}
