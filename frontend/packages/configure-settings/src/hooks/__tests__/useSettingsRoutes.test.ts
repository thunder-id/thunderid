// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import useSettingsRoutes, {defaultSettingsRoutePaths} from '../useSettingsRoutes';

const mockUseRoutes = vi.fn();

vi.mock('@thunderid/contexts', () => ({
  useRoutes: () => mockUseRoutes() as unknown,
}));

describe('useSettingsRoutes', () => {
  it('falls back to the default flow paths when the host supplies none', () => {
    mockUseRoutes.mockReturnValue({});

    const {result} = renderHook(() => useSettingsRoutes());

    expect(result.current.flows.list()).toBe('/flows');
    expect(result.current.flows.detail('flow-1')).toBe('/flows/flow-1');
    expect(defaultSettingsRoutePaths.flows.detail('flow-1')).toBe('/flows/flow-1');
  });

  it('prefers the host supplied flow paths', () => {
    mockUseRoutes.mockReturnValue({
      flows: {list: () => '/custom/flows', detail: (flowId: string) => `/custom/flows/${flowId}`},
    });

    const {result} = renderHook(() => useSettingsRoutes());

    expect(result.current.flows.list()).toBe('/custom/flows');
    expect(result.current.flows.detail('flow-2')).toBe('/custom/flows/flow-2');
  });
});
