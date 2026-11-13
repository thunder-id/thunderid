// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@testing-library/react';
import {describe, expect, it} from 'vitest';
import loadOxygenIcons from '../../utils/loadOxygenIcons';
import useOxygenIcons from '../useOxygenIcons';

describe('useOxygenIcons', () => {
  it('gives the icon set once it has loaded', async () => {
    const {result} = renderHook(() => useOxygenIcons());
    const icons = await loadOxygenIcons();

    await waitFor(() => expect(result.current).toBe(icons), {timeout: 30000});
  }, 30000);

  it('gives the already loaded set on the first render', async () => {
    const icons = await loadOxygenIcons();
    const {result} = renderHook(() => useOxygenIcons());

    expect(result.current).toBe(icons);
  });
});
