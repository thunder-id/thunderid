// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import useApplicationRoutes, {defaultApplicationRoutePaths} from '../useApplicationRoutes';

const mockUseRoutes = vi.fn();

vi.mock('@thunderid/contexts', () => ({
  useRoutes: () => mockUseRoutes() as unknown,
}));

describe('useApplicationRoutes', () => {
  describe('defaults', () => {
    it('should fall back to the default paths when the host supplies none', () => {
      mockUseRoutes.mockReturnValue({});

      const {result} = renderHook(() => useApplicationRoutes());

      expect(result.current.applications.list()).toBe('/applications');
      expect(result.current.applications.create()).toBe('/applications/create');
      expect(result.current.applications.types()).toBe('/applications/types');
      expect(result.current.applications.detail('app-1')).toBe('/applications/app-1');
    });

    it('should expose the same defaults through defaultApplicationRoutePaths', () => {
      expect(defaultApplicationRoutePaths.applications.list()).toBe('/applications');
      expect(defaultApplicationRoutePaths.applications.create()).toBe('/applications/create');
      expect(defaultApplicationRoutePaths.applications.types()).toBe('/applications/types');
      expect(defaultApplicationRoutePaths.applications.detail('app-1')).toBe('/applications/app-1');
    });
  });

  describe('host overrides', () => {
    it('should prefer the host supplied paths when present', () => {
      mockUseRoutes.mockReturnValue({
        applications: {
          list: () => '/custom/applications',
          create: () => '/custom/applications/new',
          types: () => '/custom/applications/types',
          detail: (id: string) => `/custom/applications/${id}`,
        },
      });

      const {result} = renderHook(() => useApplicationRoutes());

      expect(result.current.applications.list()).toBe('/custom/applications');
      expect(result.current.applications.create()).toBe('/custom/applications/new');
      expect(result.current.applications.types()).toBe('/custom/applications/types');
      expect(result.current.applications.detail('app-2')).toBe('/custom/applications/app-2');
    });
  });
});
