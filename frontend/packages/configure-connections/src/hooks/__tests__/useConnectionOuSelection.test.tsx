// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useConnectionOuSelection from '../useConnectionOuSelection';

const ouState = {hasMultipleOUs: false, isLoading: false, isError: false, ouList: [{id: 'ou-default'}]};

vi.mock('@thunderid/configure-organization-units', () => ({
  useHasMultipleOUs: () => ouState,
}));

describe('useConnectionOuSelection', () => {
  beforeEach(() => {
    ouState.hasMultipleOUs = false;
    ouState.isLoading = false;
    ouState.isError = false;
    ouState.ouList = [{id: 'ou-default'}];
  });

  it('implies the only organization unit without asking', () => {
    const {result} = renderHook(() => useConnectionOuSelection());

    expect(result.current.ouId).toBe('ou-default');
    expect(result.current.needsPicker).toBe(false);
  });

  it('leaves the id empty and asks for a pick when there are several organization units', () => {
    ouState.hasMultipleOUs = true;
    ouState.ouList = [{id: 'ou-a'}, {id: 'ou-b'}];
    const {result} = renderHook(() => useConnectionOuSelection());

    expect(result.current.ouId).toBe('');
    expect(result.current.needsPicker).toBe(true);

    act(() => result.current.setOuId('ou-b'));

    expect(result.current.ouId).toBe('ou-b');
  });

  it('leaves the id empty when there is no organization unit', () => {
    ouState.ouList = [];
    const {result} = renderHook(() => useConnectionOuSelection());

    expect(result.current.ouId).toBe('');
  });

  it('does not imply the only organization unit when a lookup failed', () => {
    ouState.isError = true;
    const {result} = renderHook(() => useConnectionOuSelection());

    expect(result.current.ouId).toBe('');
  });
});
