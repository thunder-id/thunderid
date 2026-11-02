// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TFunction} from 'i18next';
import {describe, it, expect} from 'vitest';
import type {AccountLockForm} from '../accountLockPayload';
import describePolicy, {formatDurationShort} from '../describePolicy';

/**
 * A stand-in for the real catalog, resolving the keys this helper uses. A key without the settings
 * namespace resolves to itself, so a sentence that drops the prefix fails its assertion.
 */
const translate = (key: string, options?: Record<string, unknown>): string => {
  const catalog: Record<string, string> = {
    'accountLock.summary.off.user': 'Automatic locking is off. Failed sign-in attempts do not lock users.',
    'accountLock.summary.off.agent': 'Automatic locking is off. Failed authentication attempts do not lock agents.',
    'accountLock.units.short.second': '{{count}} second',
    'accountLock.units.short.seconds': '{{count}} seconds',
    'accountLock.units.short.minute': '{{count}} minute',
    'accountLock.units.short.minutes': '{{count}} minutes',
    'accountLock.units.short.hour': '{{count}} hour',
    'accountLock.units.short.hours': '{{count}} hours',
    'accountLock.units.short.day': '{{count}} day',
    'accountLock.units.short.days': '{{count}} days',
    'accountLock.summary.trigger.authentication_method.user':
      'After {{count}} failed attempts with no more than {{window}} between them, the affected sign-in method is locked.',
    'accountLock.summary.trigger.authentication_method.agent':
      'After {{count}} failed attempts with no more than {{window}} between them, the affected authentication method is locked.',
    'accountLock.summary.trigger.account.user':
      'After {{count}} failed attempts with no more than {{window}} between them, all sign-in methods are locked.',
    'accountLock.summary.trigger.account.agent':
      'After {{count}} failed attempts with no more than {{window}} between them, all authentication methods are locked.',
    'accountLock.summary.triggerNoWindow.authentication_method.user':
      'After {{count}} failed attempts without a successful sign-in, the affected sign-in method is locked.',
    'accountLock.summary.triggerNoWindow.authentication_method.agent':
      'After {{count}} failed attempts without a successful authentication, the affected authentication method is locked.',
    'accountLock.summary.triggerNoWindow.account.user':
      'After {{count}} failed attempts without a successful sign-in, all sign-in methods are locked.',
    'accountLock.summary.triggerNoWindow.account.agent':
      'After {{count}} failed attempts without a successful authentication, all authentication methods are locked.',
    'accountLock.summary.duration.single': 'Each lockout lasts {{duration}}.',
    'accountLock.summary.duration.range':
      'The first lockout lasts {{from}}, increasing to {{to}} for subsequent lockouts.',
    'accountLock.summary.duration.permanentOnly.user':
      'Each lockout lasts until account recovery or administrator intervention.',
    'accountLock.summary.duration.permanentOnly.agent': 'Each lockout lasts until administrator intervention.',
    'accountLock.summary.duration.permanentRange.user':
      'The first lockout lasts {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
    'accountLock.summary.duration.permanentRange.agent':
      'The first lockout lasts {{from}}. Subsequent lockouts last until administrator intervention.',
    'accountLock.summary.duration.rangeThenPermanent.user':
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until account recovery or administrator intervention.',
    'accountLock.summary.duration.rangeThenPermanent.agent':
      'The first {{count}} lockouts end on their own, starting at {{from}}. Subsequent lockouts last until administrator intervention.',
    'accountLock.summary.duration.mixed.user':
      'Some lockouts last until account recovery or administrator intervention, and the others end on their own.',
    'accountLock.summary.duration.mixed.agent':
      'Some lockouts last until administrator intervention, and the others end on their own.',
  };
  const name: string | undefined = key.startsWith('settings:') ? key.slice('settings:'.length) : undefined;
  return Object.entries(options ?? {}).reduce<string>(
    (text, [variable, value]) => text.replace(`{{${variable}}}`, String(value)),
    (name !== undefined ? catalog[name] : undefined) ?? key,
  );
};
const t = translate as unknown as TFunction;

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
    expect(describePolicy(form(), 'user', t)).toBe(
      'After 5 failed attempts with no more than 15 minutes between them, the affected sign-in method is locked. ' +
        'The first lockout lasts 5 minutes, increasing to 1 day for subsequent lockouts.',
    );
  });

  // The only clause the granularity changes, and the one an operator is most likely to get wrong.
  it('says which sign-in methods an account-wide policy holds', () => {
    expect(describePolicy(form({granularity: 'account'}), 'user', t)).toContain('all sign-in methods are locked');
  });

  it('describes a single duration as a duration, not a range', () => {
    expect(describePolicy(form({lockDurationsSeconds: [900]}), 'user', t)).toContain('Each lockout lasts 15 minutes.');
  });

  // "Increasing from 15 minutes to 15 minutes" is not an escalation.
  it('describes a flat ladder as a single duration', () => {
    expect(describePolicy(form({lockDurationsSeconds: [900, 900]}), 'user', t)).toContain(
      'Each lockout lasts 15 minutes.',
    );
  });

  // A rung of zero is held until an operator unlocks, so there is no upper duration to name.
  it('says when the last lock does not lift on its own', () => {
    expect(describePolicy(form({lockDurationsSeconds: [300, 0]}), 'user', t)).toContain(
      'The first lockout lasts 5 minutes. Subsequent lockouts last until account recovery or administrator intervention.',
    );
    expect(describePolicy(form({lockDurationsSeconds: [0]}), 'user', t)).toContain(
      'Each lockout lasts until account recovery or administrator intervention.',
    );
  });

  // "Subsequent lockouts last until" would be wrong once a second timed lock comes before the held ones.
  it('counts the timed locks before a held one', () => {
    expect(describePolicy(form({lockDurationsSeconds: [60, 300, 0]}), 'user', t)).toContain(
      'The first 2 lockouts end on their own, starting at 1 minute. Subsequent lockouts last until account recovery or administrator intervention.',
    );
  });

  it('says every lock is held when no duration lifts on its own', () => {
    expect(describePolicy(form({lockDurationsSeconds: [0, 0]}), 'user', t)).toContain(
      'Each lockout lasts until account recovery or administrator intervention.',
    );
  });

  // A zero before the last duration is valid; naming a first and last duration around it would mislead.
  it('does not name a range around a held lock in the middle', () => {
    expect(describePolicy(form({lockDurationsSeconds: [300, 0, 900]}), 'user', t)).toContain(
      'Some lockouts last until account recovery or administrator intervention, and the others end on their own.',
    );
    expect(describePolicy(form({lockDurationsSeconds: [0, 300]}), 'user', t)).toContain('Some lockouts last');
  });

  // The shipped agent policy starts at one minute.
  it('states the shipped agent ladder in the singular where it is one', () => {
    expect(describePolicy(form({lockDurationsSeconds: [60, 300, 900]}), 'user', t)).toContain(
      'The first lockout lasts 1 minute, increasing to 15 minutes for subsequent lockouts.',
    );
  });

  // An agent authenticates rather than signs in, so its card names the method that way.
  it('names the authentication method on the agent card', () => {
    expect(describePolicy(form({lockDurationsSeconds: [60, 300, 900]}), 'agent', t)).toBe(
      'After 5 failed attempts with no more than 15 minutes between them, the affected authentication method is locked. ' +
        'The first lockout lasts 1 minute, increasing to 15 minutes for subsequent lockouts.',
    );
    expect(describePolicy(form({granularity: 'account'}), 'agent', t)).toContain(
      'all authentication methods are locked',
    );
    // An agent has no account recovery, so only an administrator ends a held lockout.
    expect(describePolicy(form({lockDurationsSeconds: [60, 0]}), 'agent', t)).toContain(
      'Subsequent lockouts last until administrator intervention.',
    );
    expect(describePolicy(form({failureWindowSeconds: 0}), 'agent', t)).toContain(
      'without a successful authentication, the affected authentication method is locked.',
    );
  });

  // With the switch off the card hides every setting, so the summary is all that says what that means.
  it('says that failed attempts lock nothing while automatic locking is off', () => {
    expect(describePolicy(form({enabled: false}), 'user', t)).toBe(
      'Automatic locking is off. Failed sign-in attempts do not lock users.',
    );
    expect(describePolicy(form({enabled: false}), 'agent', t)).toBe(
      'Automatic locking is off. Failed authentication attempts do not lock agents.',
    );
    // An incomplete policy is still off, not unstated.
    expect(describePolicy(form({enabled: false, threshold: 0, lockDurationsSeconds: []}), 'user', t)).toBe(
      'Automatic locking is off. Failed sign-in attempts do not lock users.',
    );
  });

  // The field validation already says what is missing, and a sentence built around a missing number
  // would be a worse way of saying the same thing.
  it('says nothing about a policy that is not complete', () => {
    expect(describePolicy(form({lockDurationsSeconds: []}), 'user', t)).toBe('');
    expect(describePolicy(form({threshold: 0}), 'user', t)).toBe('');
  });

  // A window of zero never resets the count on time, so the sentence names what does reset it.
  it('states that a zero window counts until a successful sign-in', () => {
    expect(describePolicy(form({failureWindowSeconds: 0}), 'user', t)).toBe(
      'After 5 failed attempts without a successful sign-in, the affected sign-in method is locked. ' +
        'The first lockout lasts 5 minutes, increasing to 1 day for subsequent lockouts.',
    );
    expect(describePolicy(form({failureWindowSeconds: 0, granularity: 'account'}), 'user', t)).toContain(
      'without a successful sign-in, all sign-in methods are locked.',
    );
  });
});
