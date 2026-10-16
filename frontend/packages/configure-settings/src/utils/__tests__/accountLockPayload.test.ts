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
      notifyEnabled: false,
      notifyRecipientAttribute: 'email',
      recordLastLogin: false,
      lastLoginResolutionSeconds: 3600,
    });
  });

  // The server records nothing unless told to, and refreshes hourly once on.
  it('reads unset sign-in activity as off with the server default interval', () => {
    const form = toAccountLockForm({user: {default: {threshold: 5}}}, 'agent');
    expect(form.recordLastLogin).toBe(false);
    expect(form.lastLoginResolutionSeconds).toBe(3600);
    expect(toAccountLockForm({agent: {recordLastLogin: true, lastLoginResolutionSeconds: 0}}, 'agent')).toMatchObject({
      recordLastLogin: true,
      lastLoginResolutionSeconds: 0,
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

  it('preserves saved fields of the default block it did not change', () => {
    const saved: AccountAccessValue = {user: {default: {lockDecaySeconds: 3600}}};
    const payload = buildAccountLockPayload(saved, baseline(), forms({threshold: 4}), effective);
    expect(payload.user?.default).toEqual({lockDecaySeconds: 3600, threshold: 4});
  });

  // The lock cards and the sign-in activity card save one document, so a save from either must keep
  // what the other wrote. Dropping recording would leave a dormancy report quietly going stale.
  it('keeps saved sign-in activity when only the lock policy changes', () => {
    const saved: AccountAccessValue = {user: {recordLastLogin: true, lastLoginResolutionSeconds: 300}};
    const payload = buildAccountLockPayload(saved, baseline(), forms({threshold: 4}), effective);
    expect(payload.user).toEqual({recordLastLogin: true, lastLoginResolutionSeconds: 300, default: {threshold: 4}});
  });

  it('keeps a saved lock policy when only sign-in activity changes', () => {
    const saved: AccountAccessValue = {agent: {default: {threshold: 3}}};
    const payload = buildAccountLockPayload(saved, baseline(), forms({}, {recordLastLogin: true}), effective);
    expect(payload).toEqual({agent: {default: {threshold: 3}, recordLastLogin: true}});
  });

  it('writes each sign-in activity field only when it changed', () => {
    expect(
      buildAccountLockPayload(
        undefined,
        baseline(),
        forms({recordLastLogin: true, lastLoginResolutionSeconds: 0}),
        effective,
      ),
    ).toEqual({user: {recordLastLogin: true, lastLoginResolutionSeconds: 0}});
    expect(buildAccountLockPayload(undefined, baseline(), forms({recordLastLogin: false}), effective)).toEqual({});
  });

  it('writes the granularity only when it changed', () => {
    expect(buildAccountLockPayload(undefined, baseline(), forms({granularity: 'entity'}), effective)).toEqual({
      user: {lockGranularity: 'entity'},
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

  // Not editable from this screen, and a save that dropped them would remove an opt-out an operator
  // configured through the API.
  it('preserves the recipient attribute it does not edit', () => {
    const saved = {user: {notifications: {onLock: {email: {recipientAttribute: 'workEmail'}}}}};

    const payload = buildAccountLockPayload(saved, baseline(), forms({notifyEnabled: true}), effective);

    expect(payload.user?.notifications?.onLock?.email).toEqual({recipientAttribute: 'workEmail', enabled: true});
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
  it('tells sign-in activity edits apart', () => {
    const base = toAccountLockForm(effective, 'user');
    expect(isSameAccountLockForm(base, {...base, recordLastLogin: true})).toBe(false);
    expect(isSameAccountLockForm(base, {...base, lastLoginResolutionSeconds: 300})).toBe(false);
  });

  it('compares the ladder entry by entry', () => {
    const base = toAccountLockForm(effective, 'user');
    expect(isSameAccountLockForm(base, toAccountLockForm(effective, 'user'))).toBe(true);
    expect(isSameAccountLockForm(base, {...base, lockDurationsSeconds: [300, 900, 3600]})).toBe(false);
    expect(isSameAccountLockForm(base, {...base, lockDurationsSeconds: [300, 900, 3600, 86401]})).toBe(false);
  });
});

describe('recipient attribute', () => {
  it('writes the attribute only when it changes', () => {
    const loaded = forms();
    loaded.user = toAccountLockForm({user: {notifications: {onLock: {email: {recipientAttribute: 'email'}}}}}, 'user');
    const edited = structuredClone(loaded);
    edited.user.notifyRecipientAttribute = 'workEmail';
    expect(buildAccountLockPayload({}, loaded, loaded, effective)).toEqual({});
    expect(buildAccountLockPayload({}, loaded, edited, effective)).toEqual({
      user: {notifications: {onLock: {email: {recipientAttribute: 'workEmail'}}}},
    });
    expect(isSameAccountLockForm(loaded.user, edited.user)).toBe(false);
  });
});
