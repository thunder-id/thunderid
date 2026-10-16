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
            recipientAttribute: 'email',
          },
        },
      },
    },
  },
};

describe('account lock recipient draft', () => {
  it('requires a recipient attribute, then saves and resets valid edits', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    act(() => result.current.change('user', {notifyRecipientAttribute: ' '}));
    expect(result.current.errors.user.notifyRecipientAttribute).toBeTruthy();
    act(() => result.current.change('user', {notifyRecipientAttribute: 'workEmail'}));
    expect(result.current.hasErrors).toBe(false);
    expect(result.current.buildPayload()).toEqual({
      user: {notifications: {onLock: {email: {recipientAttribute: 'workEmail'}}}},
    });
    act(() => result.current.reset());
    expect(result.current.dirty).toBe(false);
    expect(result.current.buildPayload()).toEqual({});
  });

  it('accepts an empty recipient while the card or its email is switched off', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    act(() => result.current.change('user', {notifyRecipientAttribute: '', notifyEnabled: false}));
    expect(result.current.errors.user.notifyRecipientAttribute).toBeUndefined();
    act(() => result.current.change('user', {notifyEnabled: true, enabled: false}));
    expect(result.current.errors.user.notifyRecipientAttribute).toBeUndefined();
    act(() => result.current.change('user', {enabled: true}));
    expect(result.current.errors.user.notifyRecipientAttribute).toBeTruthy();
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

describe('sign-in activity draft', () => {
  // One section, one save: an edit on either card reaches the same payload and the same reset.
  it('saves sign-in activity and lock edits together', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    act(() => result.current.change('agent', {recordLastLogin: true, lastLoginResolutionSeconds: 300}));
    expect(result.current.dirty).toBe(true);
    act(() => result.current.change('user', {threshold: 3}));

    expect(result.current.hasErrors).toBe(false);
    expect(result.current.buildPayload()).toEqual({
      user: {default: {threshold: 3}},
      agent: {recordLastLogin: true, lastLoginResolutionSeconds: 300},
    });

    act(() => result.current.reset());
    expect(result.current.forms.agent.recordLastLogin).toBe(false);
    expect(result.current.dirty).toBe(false);
  });

  // Turning locking off must not turn recording off, or the other way round.
  it('keeps recording on while the lock is switched off', () => {
    const {result} = renderHook(() => useAccountLockDraft(data));
    act(() => result.current.change('user', {recordLastLogin: true}));
    act(() => result.current.change('user', {enabled: false}));

    expect(result.current.buildPayload()).toEqual({user: {default: {enabled: false}, recordLastLogin: true}});
  });
});
