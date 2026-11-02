// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import useFlowRoutes, {defaultFlowRoutePaths} from '../useFlowRoutes';

const mockUseRoutes = vi.fn();

vi.mock('@thunderid/contexts', () => ({
  useRoutes: () => mockUseRoutes() as unknown,
}));

describe('useFlowRoutes', () => {
  describe('defaults', () => {
    it('should fall back to the default paths when the host supplies none', () => {
      mockUseRoutes.mockReturnValue({});

      const {result} = renderHook(() => useFlowRoutes());

      expect(result.current.flows.list()).toBe('/flows');
      expect(result.current.flows.create()).toBe('/flows/create');
      expect(result.current.flows.detail('flow-1')).toBe('/flows/flow-1');
    });

    it('should expose the same defaults through defaultFlowRoutePaths', () => {
      expect(defaultFlowRoutePaths.flows.list()).toBe('/flows');
      expect(defaultFlowRoutePaths.flows.create()).toBe('/flows/create');
      expect(defaultFlowRoutePaths.flows.detail('flow-1')).toBe('/flows/flow-1');
    });
  });

  describe('host overrides', () => {
    it('should prefer the host supplied paths when present', () => {
      mockUseRoutes.mockReturnValue({
        flows: {
          list: () => '/custom/flows',
          create: () => '/custom/flows/new',
          detail: (flowId: string) => `/custom/flows/${flowId}`,
        },
      });

      const {result} = renderHook(() => useFlowRoutes());

      expect(result.current.flows.list()).toBe('/custom/flows');
      expect(result.current.flows.create()).toBe('/custom/flows/new');
      expect(result.current.flows.detail('flow-2')).toBe('/custom/flows/flow-2');
    });
  });
});
