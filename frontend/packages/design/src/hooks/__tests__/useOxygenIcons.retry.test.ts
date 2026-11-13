// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@testing-library/react';
import {describe, expect, it, vi} from 'vitest';
import loadOxygenIcons from '../../utils/loadOxygenIcons';
import useOxygenIcons from '../useOxygenIcons';

// The first load of the icon data fails, as a failed chunk download would; later loads succeed.
// The failure is raised when the loader reads the data, not when the module is evaluated: a module
// that throws while it is evaluated cannot be imported again, unlike a chunk that failed to download.
const data = vi.hoisted(() => ({failuresLeft: 1}));
vi.mock('../../utils/oxygenIconData', async (importOriginal) => {
  const actual = await importOriginal<typeof import('../../utils/oxygenIconData')>();
  return {
    default: new Proxy(actual.default, {
      get(target, property, receiver) {
        if (property === 'icons' && data.failuresLeft > 0) {
          data.failuresLeft -= 1;
          throw new Error('Failed to fetch the icon data');
        }
        return Reflect.get(target, property, receiver) as unknown;
      },
    }),
  };
});

describe('useOxygenIcons after a failed load', () => {
  it('gives a mounted component the icon set once another caller loads it', async () => {
    const {result} = renderHook(() => useOxygenIcons());

    // The component's own load fails.
    await expect(loadOxygenIcons()).rejects.toThrow('Failed to fetch the icon data');
    expect(result.current).toBeUndefined();

    // Another caller retries, and the still-mounted component gets the set.
    const icons = await loadOxygenIcons();
    await waitFor(() => expect(result.current).toBe(icons));
  }, 30000);
});
