// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, render, screen} from '@thunderid/test-utils';
import type {JSX} from 'react';
import {afterEach, describe, expect, it, vi} from 'vitest';
import useNow from '../useNow';

function Clock(): JSX.Element {
  return <span>{useNow(1000)}</span>;
}

describe('useNow', () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it('shares one timer between every component asking for the same interval, and stops it after the last', () => {
    const setIntervalSpy = vi.spyOn(globalThis, 'setInterval');
    const clearIntervalSpy = vi.spyOn(globalThis, 'clearInterval');

    const {unmount} = render(
      <>
        <Clock />
        <Clock />
        <Clock />
      </>,
    );

    expect(setIntervalSpy.mock.calls.filter(([, delay]) => delay === 1000)).toHaveLength(1);

    unmount();

    expect(clearIntervalSpy).toHaveBeenCalledTimes(1);
  });

  it('re-renders with the new time on every tick', () => {
    vi.useFakeTimers();
    try {
      vi.setSystemTime(new Date('2026-01-01T00:00:00Z'));
      render(<Clock />);
      const before = Number(screen.getByText(/^\d+$/).textContent);

      act(() => {
        vi.advanceTimersByTime(1000);
      });

      expect(Number(screen.getByText(/^\d+$/).textContent)).toBe(before + 1000);
    } finally {
      vi.useRealTimers();
    }
  });
});
