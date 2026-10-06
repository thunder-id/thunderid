// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AccountLockForm} from '../accountLockPayload';
import describePolicy, {formatDurationShort} from '../describePolicy';

/** A stand-in for the real catalog, resolving the keys this helper uses. */
const t = (key: string, options?: Record<string, unknown>): string => {
  const catalog: Record<string, string> = {
    'accountLock.units.short.second': '{{count}} second',
    'accountLock.units.short.seconds': '{{count}} seconds',
    'accountLock.units.short.minute': '{{count}} minute',
    'accountLock.units.short.minutes': '{{count}} minutes',
    'accountLock.units.short.hour': '{{count}} hour',
    'accountLock.units.short.hours': '{{count}} hours',
    'accountLock.units.short.day': '{{count}} day',
    'accountLock.units.short.days': '{{count}} days',
    'accountLock.summary.trigger.authentication_method':
      'After {{count}} failed attempts within {{window}}, the affected sign-in method is locked.',
    'accountLock.summary.trigger.entity':
      'After {{count}} failed attempts within {{window}}, every sign-in method is locked.',
    'accountLock.summary.triggerNoWindow.authentication_method':
      'After {{count}} failed attempts without a successful sign-in, the affected sign-in method is locked.',
    'accountLock.summary.triggerNoWindow.entity':
      'After {{count}} failed attempts without a successful sign-in, every sign-in method is locked.',
    'accountLock.summary.duration.single': 'Each lock lasts {{duration}}.',
    'accountLock.summary.duration.range': 'The first lock lasts {{from}}, increasing to {{to}} for repeated locks.',
    'accountLock.summary.duration.permanentOnly': 'A lock is held until an administrator unlocks it.',
    'accountLock.summary.duration.permanentRange':
      'The first lock lasts {{from}}, and repeated locks are held until an administrator unlocks.',
    'accountLock.summary.duration.rangeThenPermanent':
      'The first {{count}} locks lift on their own, starting at {{from}}. Later locks are held until an administrator unlocks.',
    'accountLock.summary.duration.mixed':
      'Some locks are held until an administrator unlocks them, and the others lift on their own.',
  };
  return Object.entries(options ?? {}).reduce<string>(
    (text, [name, value]) => text.replace(`{{${name}}}`, String(value)),
    catalog[key] ?? key,
  );
};

const form = (overrides: Partial<AccountLockForm> = {}): AccountLockForm => ({
  enabled: true,
  granularity: 'authentication_method',
  threshold: 5,
  failureWindowSeconds: 900,
  lockDurationsSeconds: [300, 900, 3600, 86400],
  lockDecaySeconds: 86400,
  maxConsecutiveFailures: 50,
  notifyEnabled: false,
  ...overrides,
});

describe('formatDurationShort', () => {
  it('inflects the units that inflect and leaves the ones that do not', () => {
    expect(formatDurationShort(3600, t)).toBe('1 hour');
    expect(formatDurationShort(7200, t)).toBe('2 hours');
    expect(formatDurationShort(86400, t)).toBe('1 day');
    expect(formatDurationShort(172800, t)).toBe('2 days');
    expect(formatDurationShort(60, t)).toBe('1 minute');
    expect(formatDurationShort(900, t)).toBe('15 minutes');
  });
});

describe('describePolicy', () => {
  // The question four number fields leave hanging: "five failed attempts and then it locks, but for
  // how long, and does the rest of the account still work?"
  it('states the trigger and the escalation as one sentence each', () => {
    expect(describePolicy(form(), t)).toBe(
      'After 5 failed attempts within 15 minutes, the affected sign-in method is locked. ' +
        'The first lock lasts 5 minutes, increasing to 1 day for repeated locks.',
    );
  });

  // The only clause the granularity changes, and the one an operator is most likely to get wrong.
  it('says which sign-in methods an entity-wide policy holds', () => {
    expect(describePolicy(form({granularity: 'entity'}), t)).toContain('every sign-in method is locked');
  });

  it('describes a single duration as a duration, not a range', () => {
    expect(describePolicy(form({lockDurationsSeconds: [900]}), t)).toContain('Each lock lasts 15 minutes.');
  });

  // "Increasing from 15 minutes to 15 minutes" is not an escalation.
  it('describes a flat ladder as a single duration', () => {
    expect(describePolicy(form({lockDurationsSeconds: [900, 900]}), t)).toContain('Each lock lasts 15 minutes.');
  });

  // A rung of zero is held until an operator unlocks, so there is no upper duration to name.
  it('says when the last lock does not lift on its own', () => {
    expect(describePolicy(form({lockDurationsSeconds: [300, 0]}), t)).toContain(
      'The first lock lasts 5 minutes, and repeated locks are held until an administrator unlocks.',
    );
    expect(describePolicy(form({lockDurationsSeconds: [0]}), t)).toContain(
      'A lock is held until an administrator unlocks it.',
    );
  });

  // "Repeated locks are held" would be wrong once a second timed lock comes before the held ones.
  it('counts the timed locks before a held one', () => {
    expect(describePolicy(form({lockDurationsSeconds: [60, 300, 0]}), t)).toContain(
      'The first 2 locks lift on their own, starting at 1 minute. Later locks are held until an administrator unlocks.',
    );
  });

  it('says every lock is held when no duration lifts on its own', () => {
    expect(describePolicy(form({lockDurationsSeconds: [0, 0]}), t)).toContain(
      'A lock is held until an administrator unlocks it.',
    );
  });

  // A zero before the last duration is valid; naming a first and last duration around it would mislead.
  it('does not name a range around a held lock in the middle', () => {
    expect(describePolicy(form({lockDurationsSeconds: [300, 0, 900]}), t)).toContain(
      'Some locks are held until an administrator unlocks them, and the others lift on their own.',
    );
    expect(describePolicy(form({lockDurationsSeconds: [0, 300]}), t)).toContain('Some locks are held');
  });

  // The shipped agent policy starts at one minute.
  it('states the shipped agent ladder in the singular where it is one', () => {
    expect(describePolicy(form({lockDurationsSeconds: [60, 300, 900]}), t)).toContain(
      'The first lock lasts 1 minute, increasing to 15 minutes for repeated locks.',
    );
  });

  // The field validation already says what is missing, and a sentence built around a missing number
  // would be a worse way of saying the same thing.
  it('says nothing about a policy that is not complete', () => {
    expect(describePolicy(form({lockDurationsSeconds: []}), t)).toBe('');
    expect(describePolicy(form({threshold: 0}), t)).toBe('');
  });

  // A window of zero never resets the count on time, so the sentence names what does reset it.
  it('states that a zero window counts until a successful sign-in', () => {
    expect(describePolicy(form({failureWindowSeconds: 0}), t)).toBe(
      'After 5 failed attempts without a successful sign-in, the affected sign-in method is locked. ' +
        'The first lock lasts 5 minutes, increasing to 1 day for repeated locks.',
    );
    expect(describePolicy(form({failureWindowSeconds: 0, granularity: 'entity'}), t)).toContain(
      'without a successful sign-in, every sign-in method is locked.',
    );
  });
});
