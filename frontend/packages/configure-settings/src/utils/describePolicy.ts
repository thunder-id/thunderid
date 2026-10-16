// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccountLockForm} from './accountLockPayload';
import {toDurationParts, type DurationParts, type DurationUnit} from './duration';

/**
 * A minimal subset of the i18next translate function, so this stays testable without i18n.
 */
type TranslateFn = (key: string, options?: Record<string, unknown>) => string;

/** Fallback copy for the dynamic keys, matching en-US. */
const SHORT_UNIT_FALLBACKS: Record<string, string> = {
  second: '{{count}} second',
  seconds: '{{count}} seconds',
  minute: '{{count}} minute',
  minutes: '{{count}} minutes',
  hour: '{{count}} hour',
  hours: '{{count}} hours',
  day: '{{count}} day',
  days: '{{count}} days',
};

const TRIGGER_FALLBACKS: Record<AccountLockForm['granularity'], {window: string; noWindow: string}> = {
  authentication_method: {
    window: 'After {{count}} failed attempts within {{window}}, the affected sign-in method is locked.',
    noWindow: 'After {{count}} failed attempts without a successful sign-in, the affected sign-in method is locked.',
  },
  entity: {
    window: 'After {{count}} failed attempts within {{window}}, every sign-in method is locked.',
    noWindow: 'After {{count}} failed attempts without a successful sign-in, every sign-in method is locked.',
  },
};

/** Units whose short form inflects, and the key suffix to use for one versus many. */
const SHORT_UNIT_KEYS: Record<DurationUnit, {one: string; other: string}> = {
  seconds: {one: 'second', other: 'seconds'},
  minutes: {one: 'minute', other: 'minutes'},
  hours: {one: 'hour', other: 'hours'},
  days: {one: 'day', other: 'days'},
};

/**
 * Formats a duration for the summary sentence, as "5 minutes" or "1 day".
 *
 * Spelled out rather than abbreviated, because this reads inside a sentence rather than beside a
 * field; the unit dropdowns keep their own labels. Singular and plural are separate keys chosen
 * here rather than an i18next plural form, so a locale that inflects differently can say so.
 *
 * @param seconds - The duration
 * @param t - Translate function scoped so that a bare key resolves in the settings catalog
 * @returns The compact label
 *
 * @public
 */
export function formatDurationShort(seconds: number, t: TranslateFn): string {
  const parts: DurationParts = toDurationParts(seconds);
  const keys = SHORT_UNIT_KEYS[parts.unit];
  const suffix: string = parts.value === 1 ? keys.one : keys.other;
  return t(`accountLock.units.short.${suffix}`, {defaultValue: SHORT_UNIT_FALLBACKS[suffix], count: parts.value});
}

/**
 * The policy as one or two sentences an operator can check without reading the form back.
 *
 * The controls say what each number is; this says what they add up to, which is the question an
 * operator is actually deciding. Stating the outcome also makes a mistake visible: a window of 15
 * seconds reads as a plausible number in a box and as an obvious error in a sentence.
 *
 * It describes the draft rather than the saved value, so it moves as the operator edits.
 *
 * An incomplete policy has no outcome to state and returns an empty string; the field validation
 * already says what is missing.
 *
 * @param form - The values currently in the form
 * @param t - Translate function scoped so that a bare key resolves in the settings catalog
 * @returns The summary, or an empty string when there is nothing true to say
 *
 * @public
 */
export default function describePolicy(form: AccountLockForm, t: TranslateFn): string {
  const durations: number[] = form.lockDurationsSeconds;

  if (form.threshold < 1 || durations.length === 0) {
    return '';
  }

  // A window of `0` never resets the count on time, so only a successful sign-in or a lock does.
  const trigger: string =
    form.failureWindowSeconds === 0
      ? t(`accountLock.summary.triggerNoWindow.${form.granularity}`, {
          defaultValue: TRIGGER_FALLBACKS[form.granularity].noWindow,
          count: form.threshold,
        })
      : t(`accountLock.summary.trigger.${form.granularity}`, {
          defaultValue: TRIGGER_FALLBACKS[form.granularity].window,
          count: form.threshold,
          window: formatDurationShort(form.failureWindowSeconds, t),
        });

  return `${trigger} ${describeDurations(durations, t)}`;
}

/**
 * The second sentence: how long the lock lasts, and whether repeating changes that.
 *
 * A rung of `0` is held until an operator unlocks, so a ladder ending in one has no upper duration
 * to name and says so instead. A ladder whose first and last rung are equal is described as a single
 * duration, because "increasing from 15 minutes to 15 minutes" is not an escalation. A `0` before the
 * last rung is valid but unusual, and is stated without naming the rung.
 */
function describeDurations(durations: number[], t: TranslateFn): string {
  if (durations.every((seconds) => seconds === 0)) {
    return t('accountLock.summary.duration.permanentOnly', {
      defaultValue: 'A lock is held until an administrator unlocks it.',
    });
  }

  const endsPermanent: boolean = durations[durations.length - 1] === 0;
  const timed: number[] = endsPermanent ? durations.slice(0, -1) : durations;

  if (timed.includes(0)) {
    return t('accountLock.summary.duration.mixed', {
      defaultValue: 'Some locks are held until an administrator unlocks them, and the others lift on their own.',
    });
  }

  const first: number = timed[0];
  const last: number = timed[timed.length - 1];

  if (endsPermanent) {
    if (timed.length === 1) {
      return t('accountLock.summary.duration.permanentRange', {
        defaultValue: 'The first lock lasts {{from}}, and repeated locks are held until an administrator unlocks.',
        from: formatDurationShort(first, t),
      });
    }
    return t('accountLock.summary.duration.rangeThenPermanent', {
      defaultValue:
        'The first {{count}} locks lift on their own, starting at {{from}}. Later locks are held until an administrator unlocks.',
      count: timed.length,
      from: formatDurationShort(first, t),
    });
  }
  if (first === last) {
    return t('accountLock.summary.duration.single', {
      defaultValue: 'Each lock lasts {{duration}}.',
      duration: formatDurationShort(last, t),
    });
  }
  return t('accountLock.summary.duration.range', {
    defaultValue: 'The first lock lasts {{from}}, increasing to {{to}} for repeated locks.',
    from: formatDurationShort(first, t),
    to: formatDurationShort(last, t),
  });
}
