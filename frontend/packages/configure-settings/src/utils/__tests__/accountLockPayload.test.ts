// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {AccountAccessPolicy, AccountAccessValue, AccountLockCategory} from '../../models/accountAccess';
import {
  buildAccountLockPayload,
  isSameAccountLockForm,
  toAccountLockForm,
  type AccountLockForm,
} from '../accountLockPayload';

// The effective policy as a read returns it: the deployment file, which the server folds in, plus any
// section layers above it. This is what the form loads and what a save is measured against.
const effective: AccountAccessValue = {
  user: {
    lockGranularity: 'authentication_method',
    default: {
      enabled: true,
      threshold: 5,
      failureWindowSeconds: 900,
      lockDurationsSeconds: [300, 900, 3600, 86400],
      lockDecaySeconds: 86400,
      maxConsecutiveFailures: 50,
    },
    recordLastLogin: false,
    lastLoginResolutionSeconds: 3600,
  },
  agent: {
    lockGranularity: 'authentication_method',
    default: {enabled: true, threshold: 5, failureWindowSeconds: 900, lockDurationsSeconds: [60, 300, 900]},
    recordLastLogin: false,
    lastLoginResolutionSeconds: 3600,
  },
};

const forms = (
  user: Partial<AccountLockForm> = {},
  agent: Partial<AccountLockForm> = {},
): Record<AccountLockCategory, AccountLockForm> => ({
  user: {...toAccountLockForm(effective, 'user'), ...user},
  agent: {...toAccountLockForm(effective, 'agent'), ...agent},
});

/** The values as loaded, which is what a save is diffed against. */
const baseline = (): Record<AccountLockCategory, AccountLockForm> => forms();

describe('toAccountLockForm', () => {
  it('reads the effective policy', () => {
    expect(toAccountLockForm(effective, 'user')).toEqual({
      enabled: true,
      granularity: 'authentication_method',
      threshold: 5,
      failureWindowSeconds: 900,
      lockDurationsSeconds: [300, 900, 3600, 86400],
      lockDecaySeconds: 86400,
      maxConsecutiveFailures: 50,
      notifyEnabled: false,
    });
  });

  it('defaults to per-sign-in-method when the section says nothing', () => {
    expect(toAccountLockForm(undefined, 'user').granularity).toBe('authentication_method');
  });

  // The server treats an unset switch as on whenever the policy can form a lock.
  it('reads an unset switch as on only when the policy can form a lock', () => {
    const unset = (policy: AccountAccessPolicy): AccountAccessValue => ({user: {default: policy}});
    expect(
      toAccountLockForm(unset({threshold: 5, failureWindowSeconds: 900, lockDurationsSeconds: [300]}), 'user').enabled,
    ).toBe(true);
    expect(toAccountLockForm(unset({threshold: 5, lockDurationsSeconds: [300]}), 'user').enabled).toBe(false);
    expect(toAccountLockForm(undefined, 'user').enabled).toBe(false);
  });

  it('copies the ladder rather than aliasing it', () => {
    toAccountLockForm(effective, 'user').lockDurationsSeconds.push(1);
    expect(effective.user?.default?.lockDurationsSeconds).toHaveLength(4);
  });
});

describe('buildAccountLockPayload', () => {
  // The trap: writing everything the form displays would pin the deployment's current policy into
  // the runtime layer, and a later change to deployment.yaml would never reach it.
  it('writes nothing when nothing changed', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms(), effective)).toEqual({});
  });

  it('writes only the field that changed', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({threshold: 3}), effective)).toEqual({
      user: {default: {threshold: 3}},
    });
  });

  // The PUT replaces the writable layer, so dropping an untouched override would silently undo it.
  it('keeps an earlier override the operator did not touch', () => {
    const saved: AccountAccessValue = {user: {default: {threshold: 3}}};
    const withOverride: AccountAccessValue = {
      ...effective,
      user: {...effective.user, default: {...effective.user?.default, threshold: 3}},
    };
    const loaded = {
      user: toAccountLockForm(withOverride, 'user'),
      agent: toAccountLockForm(withOverride, 'agent'),
    };
    const payload = buildAccountLockPayload(
      saved,
      loaded,
      {
        ...loaded,
        user: {...loaded.user, failureWindowSeconds: 1200},
      },
      withOverride,
    );
    expect(payload.user?.default).toEqual({threshold: 3, failureWindowSeconds: 1200});
  });

  // Like every other policy field, the limit is written only when it departs from what was loaded,
  // and an unset limit reads as none.
  it('writes the consecutive-failure limit only when it changes', () => {
    expect(buildAccountLockPayload({}, baseline(), forms(), effective)).toEqual({});
    const payload = buildAccountLockPayload({}, baseline(), forms({maxConsecutiveFailures: 20}), effective);
    expect(payload.user?.default).toEqual({maxConsecutiveFailures: 20});
    expect(toAccountLockForm({user: {default: {threshold: 5}}}, 'user').maxConsecutiveFailures).toBe(0);
  });

  it('preserves saved fields of the default block it did not change', () => {
    const saved: AccountAccessValue = {user: {default: {lockDecaySeconds: 3600}}};
    const payload = buildAccountLockPayload(saved, baseline(), forms({threshold: 4}), effective);
    expect(payload.user?.default).toEqual({lockDecaySeconds: 3600, threshold: 4});
  });

  // The Console does not show sign-in activity, and the PUT replaces the writable layer, so a save
  // must carry it through. Dropping recording would leave a dormancy report quietly going stale.
  it('keeps saved sign-in activity when the lock policy changes', () => {
    const saved: AccountAccessValue = {user: {recordLastLogin: true, lastLoginResolutionSeconds: 300}};
    const payload = buildAccountLockPayload(saved, baseline(), forms({threshold: 4}), effective);
    expect(payload.user).toEqual({recordLastLogin: true, lastLoginResolutionSeconds: 300, default: {threshold: 4}});
  });

  // Nor may it pin the effective value: recording from the deployment file stays the file's to change.
  it('never writes sign-in activity the saved layer does not hold', () => {
    const recording: AccountAccessValue = {
      ...effective,
      agent: {...effective.agent, recordLastLogin: true, lastLoginResolutionSeconds: 0},
    };
    const loaded = {user: toAccountLockForm(recording, 'user'), agent: toAccountLockForm(recording, 'agent')};
    const payload = buildAccountLockPayload(
      undefined,
      loaded,
      {...loaded, agent: {...loaded.agent, threshold: 4}},
      recording,
    );
    expect(payload).toEqual({agent: {default: {threshold: 4}}});
  });

  it('writes the granularity only when it changed', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({granularity: 'account'}), effective)).toEqual({
      user: {lockGranularity: 'account'},
    });
    expect(
      buildAccountLockPayload(undefined, baseline(), forms({granularity: 'authentication_method'}), effective),
    ).toEqual({});
  });

  it('writes a switched-off category as an explicit false', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({enabled: false}), effective)).toEqual({
      user: {default: {enabled: false}},
    });
  });

  it('writes the ladder as a whole when any rung changes', () => {
    const payload = buildAccountLockPayload(
      undefined,
      baseline(),
      forms({lockDurationsSeconds: [300, 900]}),
      effective,
    );
    expect(payload.user?.default?.lockDurationsSeconds).toEqual([300, 900]);
  });

  // A permanent rung the operator did not touch must come back intact.
  it('carries a permanent rung through unchanged', () => {
    const withPermanent: AccountAccessValue = {
      ...effective,
      user: {...effective.user, default: {...effective.user?.default, lockDurationsSeconds: [300, 0]}},
    };
    const loaded = {
      user: toAccountLockForm(withPermanent, 'user'),
      agent: toAccountLockForm(withPermanent, 'agent'),
    };
    expect(buildAccountLockPayload(undefined, loaded, loaded, withPermanent)).toEqual({});
  });

  it('writes a changed ladder that keeps a permanent rung', () => {
    const withPermanent: AccountAccessValue = {
      ...effective,
      user: {...effective.user, default: {...effective.user?.default, lockDurationsSeconds: [300, 0]}},
    };
    const loaded = {
      user: toAccountLockForm(withPermanent, 'user'),
      agent: toAccountLockForm(withPermanent, 'agent'),
    };
    const payload = buildAccountLockPayload(
      undefined,
      loaded,
      {
        ...loaded,
        user: {...loaded.user, lockDurationsSeconds: [600, 0]},
      },
      withPermanent,
    );
    expect(payload.user?.default?.lockDurationsSeconds).toEqual([600, 0]);
  });

  // The notification block is section-editable as of CA8, and the same two rules apply to it.
  it('writes the notification fields only when they changed', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({notifyEnabled: true}), effective)).toEqual({
      user: {notifications: {onLock: {email: {enabled: true}}}},
    });
    expect(buildAccountLockPayload(undefined, baseline(), forms(), effective)).toEqual({});
  });

  // The templates are set only through the API, so a save from this screen must carry them as stored.
  describe('with stored lock email templates', () => {
    const templates = {
      temporary: 'authentication-method-locked',
      permanent: 'authentication-method-locked-permanent',
      account: {temporary: 'account-locked'},
      methods: {otp: {temporary: 'otp-locked'}},
    };
    const saved = (): AccountAccessValue => ({user: {notifications: {onLock: {email: {templates}}}}});

    it('keeps them when the email switch changes', () => {
      expect(buildAccountLockPayload(saved(), baseline(), forms({notifyEnabled: true}), effective)).toEqual({
        user: {notifications: {onLock: {email: {templates, enabled: true}}}},
      });
    });

    it('keeps them when the lock policy changes', () => {
      expect(buildAccountLockPayload(saved(), baseline(), forms({threshold: 3}), effective)).toEqual({
        user: {default: {threshold: 3}, notifications: {onLock: {email: {templates}}}},
      });
    });

    it('keeps them when nothing changed', () => {
      expect(buildAccountLockPayload(saved(), baseline(), forms(), effective)).toEqual(saved());
    });

    it('keeps notification keys it does not know', () => {
      const withUnknown = {
        user: {notifications: {onLock: {email: {templates}, sms: {enabled: true}}, onUnlock: {}}},
      } as unknown as AccountAccessValue;

      expect(buildAccountLockPayload(withUnknown, baseline(), forms({notifyEnabled: true}), effective)).toEqual({
        user: {notifications: {onLock: {email: {templates, enabled: true}, sms: {enabled: true}}, onUnlock: {}}},
      });
    });
  });

  // The server cannot form a lock without a window, and the shown zero ("never reset") is a choice.
  it('writes the shown window when the effective policy has none', () => {
    const noWindow: AccountAccessValue = {user: {default: {threshold: 5, lockDurationsSeconds: [300]}}};
    const loaded: AccountLockForm = toAccountLockForm(noWindow, 'user');
    const both = (user: AccountLockForm): Record<AccountLockCategory, AccountLockForm> => ({
      user,
      agent: toAccountLockForm(noWindow, 'agent'),
    });

    const payload: AccountAccessValue = buildAccountLockPayload(
      undefined,
      both(loaded),
      both({...loaded, enabled: true}),
      noWindow,
    );

    expect(payload.user?.default?.failureWindowSeconds).toBe(0);
    expect(payload.agent).toBeUndefined();
  });

  it('leaves the window unwritten when the effective policy has one', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({threshold: 3}), effective)).toEqual({
      user: {default: {threshold: 3}},
    });
  });

  it('does not mutate the saved layer', () => {
    const saved: AccountAccessValue = {user: {default: {threshold: 3}}};
    buildAccountLockPayload(saved, baseline(), forms({threshold: 9}), effective);
    expect(saved.user?.default?.threshold).toBe(3);
  });
});

describe('isSameAccountLockForm', () => {
  it('tells an email choice apart', () => {
    const base = toAccountLockForm(effective, 'user');
    expect(isSameAccountLockForm(base, {...base, notifyEnabled: true})).toBe(false);
  });

  it('compares the ladder entry by entry', () => {
    const base = toAccountLockForm(effective, 'user');
    expect(isSameAccountLockForm(base, toAccountLockForm(effective, 'user'))).toBe(true);
    expect(isSameAccountLockForm(base, {...base, lockDurationsSeconds: [300, 900, 3600]})).toBe(false);
    expect(isSameAccountLockForm(base, {...base, lockDurationsSeconds: [300, 900, 3600, 86401]})).toBe(false);
  });
});
