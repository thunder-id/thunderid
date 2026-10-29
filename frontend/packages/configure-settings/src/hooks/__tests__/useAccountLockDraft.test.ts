// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@testing-library/react';
import {describe, it, expect} from 'vitest';
import type {AccountAccessConfigResponse} from '../../models/accountAccess';
import useAccountLockDraft from '../useAccountLockDraft';

const data: AccountAccessConfigResponse = {
  readOnly: {},
  writable: {},
  merged: {
    user: {
      default: {
        enabled: true,
        threshold: 5,
        failureWindowSeconds: 900,
        lockDurationsSeconds: [300],
      },
      notifications: {
        onLock: {
          email: {
            enabled: true,
          },
        },
      },
    },
  },
};

describe('account lock email draft', () => {
  it('saves the email choice on its own and resets it', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    expect(result.current.forms.user.notifyEnabled).toBe(true);
    act(() => result.current.change('user', {notifyEnabled: false}));
    expect(result.current.dirty).toBe(true);
    expect(result.current.hasErrors).toBe(false);
    expect(result.current.buildPayload()).toEqual({user: {notifications: {onLock: {email: {enabled: false}}}}});
    act(() => result.current.reset());
    expect(result.current.dirty).toBe(false);
    expect(result.current.buildPayload()).toEqual({});
  });
});

describe('account lock threshold draft', () => {
  it('requires a whole number of at least one', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    act(() => result.current.change('user', {threshold: 1.5}));
    expect(result.current.errors.user.threshold).toBeTruthy();
    act(() => result.current.change('user', {threshold: 0}));
    expect(result.current.errors.user.threshold).toBeTruthy();
    act(() => result.current.change('user', {threshold: 3}));
    expect(result.current.errors.user.threshold).toBeUndefined();
  });
});

describe('account lock consecutive-failure draft', () => {
  // Below the threshold the first lock would already be permanent, which the server refuses.
  it('accepts no limit or one at least the threshold, and nothing else', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    for (const accepted of [0, 5, 50]) {
      act(() => result.current.change('user', {maxConsecutiveFailures: accepted}));
      expect(result.current.errors.user.maxConsecutiveFailures).toBeUndefined();
    }
    for (const refused of [4, -1, 2.5]) {
      act(() => result.current.change('user', {maxConsecutiveFailures: refused}));
      expect(result.current.errors.user.maxConsecutiveFailures).toBeTruthy();
    }
  });
});

describe('sign-in activity', () => {
  // Not shown on this tab, so a save carries the saved values through untouched.
  it('keeps saved sign-in activity when the lock is switched off', () => {
    const recording = {recordLastLogin: true, lastLoginResolutionSeconds: 300};
    const {result} = renderHook(() =>
      useAccountLockDraft({
        ...data,
        writable: {user: recording},
        merged: {...data.merged, user: {...data.merged?.user, ...recording}},
      }),
    );
    act(() => result.current.change('user', {enabled: false}));

    expect(result.current.buildPayload()).toEqual({user: {...recording, default: {enabled: false}}});
  });
});
