// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TFunction} from 'i18next';
import type {AccountLockForm} from './accountLockPayload';
import {formatDurationShort} from './duration';
import {LockGranularities, type AccountLockCategory} from '../models/accountAccess';

const TRIGGER_FALLBACKS: Record<
  AccountLockForm['granularity'],
  Record<AccountLockCategory, {window: string; noWindow: string}>
> = {
  [LockGranularities.AUTHENTICATION_METHOD]: {
    user: {
      window:
        'After {{count}} failed attempts with no more than {{window}} between them, the affected sign-in method is locked.',
      noWindow: 'After {{count}} failed attempts without a successful sign-in, the affected sign-in method is locked.',
    },
    agent: {
      window:
        'After {{count}} failed attempts with no more than {{window}} between them, the affected authentication method is locked.',
      noWindow:
        'After {{count}} failed attempts without a successful authentication, the affected authentication method is locked.',
    },
  },
  [LockGranularities.ACCOUNT]: {
    user: {
      window:
        'After {{count}} failed attempts with no more than {{window}} between them, all sign-in methods are locked.',
      noWindow: 'After {{count}} failed attempts without a successful sign-in, all sign-in methods are locked.',
    },
    agent: {
      window:
        'After {{count}} failed attempts with no more than {{window}} between them, all authentication methods are locked.',
      noWindow:
        'After {{count}} failed attempts without a successful authentication, all authentication methods are locked.',
    },
  },
};

/** Fallback copy for the sentence shown while automatic locking is off, matching en-US. */
const OFF_FALLBACKS: Record<AccountLockCategory, string> = {
  user: 'Automatic locking is off. Failed sign-in attempts do not lock users.',
  agent: 'Automatic locking is off. Failed authentication attempts do not lock agents.',
};

/** Fallback copy for the held-lockout sentences, matching en-US. An agent has no account recovery. */
const HELD_FALLBACKS: Record<
  AccountLockCategory,
  {permanentOnly: string; mixed: string; permanentRange: string; rangeThenPermanent: string}
> = {
  user: {
    permanentOnly: 'Each lockout lasts until account recovery or administrator intervention.',
    mixed: 'Some lockouts last until account recovery or administrator intervention, and the others end on their own.',
    permanentRange:
      'The first lockout lasts {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
    rangeThenPermanent:
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
  },
  agent: {
    permanentOnly: 'Each lockout lasts until administrator intervention.',
    mixed: 'Some lockouts last until administrator intervention, and the others end on their own.',
    permanentRange: 'The first lockout lasts {{from}}. Subsequent lockouts last until administrator intervention.',
    rangeThenPermanent:
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until administrator intervention.',
  },
};

/**
 * The policy as one or two sentences an operator can check without reading the form back.
 *
 * The controls say what each number is; this says what they add up to, which is the question an
 * operator is actually deciding. Stating the outcome also makes a mistake visible: a window of 15
 * seconds reads as a plausible number in a box and as an obvious error in a sentence.
 *
 * It describes the draft rather than the saved value, so it moves as the operator edits. With
 * automatic locking off, it says that failed attempts lock nothing.
 *
 * An incomplete policy has no outcome to state and returns an empty string; the field validation
 * already says what is missing.
 *
 * @param form - The values currently in the form
 * @param category - The identity category, which names the method a sign-in or an authentication
 * @param t - Translate function
 * @returns The summary, or an empty string when there is nothing true to say
 *
 * @public
 */
export default function describePolicy(form: AccountLockForm, category: AccountLockCategory, t: TFunction): string {
  if (!form.enabled) {
    return t(`settings:accountLock.summary.off.${category}`, {defaultValue: OFF_FALLBACKS[category]});
  }

  const durations: number[] = form.lockDurationsSeconds;

  if (form.threshold < 1 || durations.length === 0) {
    return '';
  }

  // A window of `0` never resets the count on time, so only a successful sign-in or a lock does.
  const fallbacks = TRIGGER_FALLBACKS[form.granularity][category];
  const trigger: string =
    form.failureWindowSeconds === 0
      ? t(`settings:accountLock.summary.triggerNoWindow.${form.granularity}.${category}`, {
          defaultValue: fallbacks.noWindow,
          count: form.threshold,
        })
      : t(`settings:accountLock.summary.trigger.${form.granularity}.${category}`, {
          defaultValue: fallbacks.window,
          count: form.threshold,
          window: formatDurationShort(form.failureWindowSeconds, t),
        });

  return `${trigger} ${describeDurations(durations, category, t)}`;
}

/**
 * The second sentence: how long the lock lasts, and whether repeating changes that.
 *
 * A rung of `0` is held until a user's account recovery or an operator unlocks, so a ladder ending in one has no upper duration
 * to name and says so instead. A ladder whose first and last rung are equal is described as a single
 * duration, because "increasing from 15 minutes to 15 minutes" is not an escalation. A `0` before the
 * last rung is valid but unusual, and is stated without naming the rung.
 */
function describeDurations(durations: number[], category: AccountLockCategory, t: TFunction): string {
  const held = HELD_FALLBACKS[category];
  if (durations.every((seconds) => seconds === 0)) {
    return t(`settings:accountLock.summary.duration.permanentOnly.${category}`, {defaultValue: held.permanentOnly});
  }

  const endsPermanent: boolean = durations[durations.length - 1] === 0;
  const timed: number[] = endsPermanent ? durations.slice(0, -1) : durations;

  if (timed.includes(0)) {
    return t(`settings:accountLock.summary.duration.mixed.${category}`, {defaultValue: held.mixed});
  }

  const first: number = timed[0];
  const last: number = timed[timed.length - 1];

  if (endsPermanent) {
    if (timed.length === 1) {
      return t(`settings:accountLock.summary.duration.permanentRange.${category}`, {
        defaultValue: held.permanentRange,
        from: formatDurationShort(first, t),
      });
    }
    return t(`settings:accountLock.summary.duration.rangeThenPermanent.${category}`, {
      defaultValue: held.rangeThenPermanent,
      count: timed.length,
      from: formatDurationShort(first, t),
    });
  }
  if (first === last) {
    return t('settings:accountLock.summary.duration.single', {
      defaultValue: 'Each lockout lasts {{duration}}.',
      duration: formatDurationShort(last, t),
    });
  }
  return t('settings:accountLock.summary.duration.range', {
    defaultValue: 'The first lockout lasts {{from}}, increasing to {{to}} for subsequent lockouts.',
    from: formatDurationShort(first, t),
    to: formatDurationShort(last, t),
  });
}
