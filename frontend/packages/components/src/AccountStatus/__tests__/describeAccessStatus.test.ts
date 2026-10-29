// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import enUS from '@thunderid/i18n/locales/en-US';
import type {AccessStatusReport} from '@thunderid/types';
import {describe, expect, it} from 'vitest';
import describeAccessStatus from '../describeAccessStatus';
import type {LifecycleCategory} from '../lifecycleAction';

const common = (enUS as unknown as {common: Record<string, string>}).common;

/**
 * Translates from the en-US catalog, recording each key with its fallback.
 */
function catalogT(): {t: (key: string, fallback: string, values?: Record<string, string>) => string; keys: string[]} {
  const keys: string[] = [];
  const t = (key: string, fallback: string, values?: Record<string, string>): string => {
    keys.push(key);
    const name = key.replace(/^common:/, '');
    // A key missing from the catalog falls back; one present must match its fallback.
    if (common[name] !== undefined) {
      expect(fallback).toBe(common[name]);
    }
    return (common[name] ?? fallback).replace(/{{(\w+)}}/g, (match, variable: string) => values?.[variable] ?? match);
  };
  return {keys, t};
}

const EXPIRES_AT = '2026-10-25T10:00:00Z';
const EXPIRES_AT_TEXT = new Date(EXPIRES_AT).toLocaleString();

const describe_ = (status: AccessStatusReport | undefined, category: LifecycleCategory = 'user') => {
  const {keys, t} = catalogT();
  return {description: describeAccessStatus(status, category, t), keys};
};

describe('describeAccessStatus', () => {
  it('reads an absent status as active', () => {
    expect(describe_(undefined).description).toEqual({
      hint: 'This account can sign in normally.',
      label: 'Active',
      title: 'Active',
      value: 'ACTIVE',
    });
  });

  it('returns null for a status this build does not know', () => {
    expect(describe_({value: 'DEACTIVATED' as never}).description).toBeNull();
  });

  it.each<[LifecycleCategory, string]>([
    ['user', 'The account is suspended and cannot sign in until an administrator unsuspends it.'],
    ['agent', 'The agent is suspended and cannot authenticate until an administrator unsuspends it.'],
    ['application', 'The application is suspended and cannot authenticate until an administrator unsuspends it.'],
  ])('keys the suspended hint by category for %s', (category, hint) => {
    const {description, keys} = describe_({value: 'SUSPENDED'}, category);

    expect(description?.hint).toBe(hint);
    expect(keys).toContain(`common:accountStatus.suspended.hint.${category}`);
  });

  it("prefixes the operator's note and omits it when none was recorded", () => {
    expect(describe_({details: {operatorNote: 'Leaked key'}, value: 'SUSPENDED'}).description?.operatorNote).toBe(
      'Operator note: Leaked key',
    );
    expect(describe_({value: 'SUSPENDED'}).description?.operatorNote).toBeUndefined();
  });

  it('says a failed-attempts lock lifts on its own only when it expires', () => {
    const timed = describe_({
      details: {lockedMethods: [{expiresAt: EXPIRES_AT, method: 'account'}], reason: 'FAILED_ATTEMPTS'},
      value: 'LOCKED',
    });
    const held = describe_({
      details: {lockedMethods: [{method: 'account'}], reason: 'FAILED_ATTEMPTS'},
      value: 'LOCKED',
    });

    expect(timed.description?.hint).toMatch(/It unlocks automatically at the time shown below\./);
    expect(held.description?.hint).toBe(
      'The account was locked after too many failed sign-in attempts. It stays locked until the user completes account recovery or an administrator unlocks it.',
    );
    expect(held.keys).toContain('common:accountStatus.locked.hint.failedAttemptsPermanent.user');
  });

  it('says when a timed lock on the whole account ends, and adds no line to a held one', () => {
    const timed = describe_({
      details: {lockedMethods: [{expiresAt: EXPIRES_AT, method: 'account'}], reason: 'FAILED_ATTEMPTS'},
      value: 'LOCKED',
    });
    const held = describe_({
      details: {lockedMethods: [{method: 'account'}], reason: 'SUSPENSION_RELEASE_LOCK'},
      value: 'LOCKED',
    });

    expect(timed.description).toMatchObject({locks: [`Locked until ${EXPIRES_AT_TEXT}`], title: 'Locked'});
    // The hint already says a held lock lasts until recovery or an unlock.
    expect(held.description).toMatchObject({locks: [], title: 'Locked'});
  });

  it('lists each locked sign-in method with when it ends, without calling the account locked', () => {
    const {description, keys} = describe_({
      details: {
        lockedMethods: [{expiresAt: EXPIRES_AT, method: 'credential'}, {method: 'otp'}],
        reason: 'FAILED_ATTEMPTS',
      },
      value: 'LOCKED',
    });

    expect(description).toMatchObject({
      label: 'Locked',
      locks: [
        `Password or other stored credential: locked until ${EXPIRES_AT_TEXT}`,
        'One-time password: locked until released',
      ],
      title: 'Sign-in locked for some methods',
    });
    expect(description?.hint).toMatch(/^Some sign-in methods were locked after too many failed attempts\./);
    expect(description?.hint).not.toMatch(/The account is locked/);
    expect(keys).toEqual(
      expect.arrayContaining([
        'common:accountStatus.locked.hint.someMethods.user',
        'common:accountStatus.locked.title.someMethods.user',
      ]),
    );
  });

  it('uses the authentication wording for an agent locked on some methods', () => {
    const {description} = describe_(
      {details: {lockedMethods: [{method: 'credential'}], reason: 'FAILED_ATTEMPTS'}, value: 'LOCKED'},
      'agent',
    );

    expect(description?.title).toBe('Authentication locked for some methods');
    expect(description?.hint).toMatch(/^Some authentication methods were locked/);
  });

  it('names a method this build has no label for by its value', () => {
    const {description} = describe_({
      details: {lockedMethods: [{method: 'newmethod'}], reason: 'FAILED_ATTEMPTS'},
      value: 'LOCKED',
    });

    expect(description?.locks).toEqual(['newmethod: locked until released']);
  });

  it('says a user left locked by an unsuspend needs recovery or an unlock', () => {
    expect(describe_({details: {reason: 'SUSPENSION_RELEASE_LOCK'}, value: 'LOCKED'}).description?.hint).toMatch(
      /^Suspension removed\. The account remains locked/,
    );
  });

  it('says a held failed-attempts lock on an agent lasts until an administrator unlocks it', () => {
    const {description} = describe_(
      {details: {lockedMethods: [{method: 'account'}], reason: 'FAILED_ATTEMPTS'}, value: 'LOCKED'},
      'agent',
    );

    expect(description?.hint).toBe(
      'The agent was locked after too many failed authentication attempts. It stays locked until an administrator unlocks it.',
    );
  });

  it('uses the authentication wording for a locked agent', () => {
    const {description, keys} = describe_({value: 'LOCKED'}, 'agent');

    expect(description?.hint).toMatch(/^The agent is locked and cannot authenticate/);
    expect(keys).toContain('common:accountStatus.locked.hint.agent');
  });

  // Applications are never locked; should one be reported, it reads as a user's lock.
  it('uses the user wording for a locked application', () => {
    const {description, keys} = describe_({value: 'LOCKED'}, 'application');

    expect(description?.hint).toBe(describe_({value: 'LOCKED'}).description?.hint);
    expect(keys).toContain('common:accountStatus.locked.hint.user');
  });
});
