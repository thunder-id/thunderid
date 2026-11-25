// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@thunderid/test-utils';
import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import useDebouncedValue from '../useDebouncedValue';

describe('useDebouncedValue', () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('starts with the value it is given', () => {
    const {result} = renderHook(() => useDebouncedValue('a', 300));

    expect(result.current).toBe('a');
  });

  it('takes a new value only once it has stayed the same for the delay', () => {
    const {result, rerender} = renderHook(({value}: {value: string}) => useDebouncedValue(value, 300), {
      initialProps: {value: 'a'},
    });

    rerender({value: 'ab'});
    act(() => {
      vi.advanceTimersByTime(200);
    });
    rerender({value: 'abc'});
    act(() => {
      vi.advanceTimersByTime(200);
    });
    expect(result.current).toBe('a');

    act(() => {
      vi.advanceTimersByTime(100);
    });
    expect(result.current).toBe('abc');
  });

  it('keeps a function value as it is, neither calling it at first nor once it settles', () => {
    const first = vi.fn(() => 'called');
    const second = vi.fn(() => 'called');
    const {result, rerender} = renderHook(({value}: {value: () => string}) => useDebouncedValue(value, 300), {
      initialProps: {value: first},
    });
    expect(result.current).toBe(first);

    rerender({value: second});
    act(() => {
      vi.advanceTimersByTime(300);
    });

    expect(result.current).toBe(second);
    expect(first).not.toHaveBeenCalled();
    expect(second).not.toHaveBeenCalled();
  });
});
