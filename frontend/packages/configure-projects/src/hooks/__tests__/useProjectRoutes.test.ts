// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@testing-library/react';
import {describe, it, expect, vi} from 'vitest';
import useProjectRoutes from '../useProjectRoutes';

const mockUseRoutes = vi.fn();

vi.mock('@thunderid/contexts', () => ({
  useRoutes: () => mockUseRoutes() as unknown,
}));

describe('useProjectRoutes', () => {
  it('falls back to the default paths when the host supplies none', () => {
    mockUseRoutes.mockReturnValue({});

    const {result} = renderHook(() => useProjectRoutes());

    expect(result.current.projects.list()).toBe('/projects');
    expect(result.current.projects.create()).toBe('/projects/create');
    expect(result.current.applications.list()).toBe('/applications');
  });

  it('prefers the paths the host supplies', () => {
    mockUseRoutes.mockReturnValue({
      projects: {list: () => '/custom/projects', create: () => '/custom/projects/new'},
      applications: {list: () => '/custom/apps'},
    });

    const {result} = renderHook(() => useProjectRoutes());

    expect(result.current.projects.list()).toBe('/custom/projects');
    expect(result.current.projects.create()).toBe('/custom/projects/new');
    expect(result.current.applications.list()).toBe('/custom/apps');
  });
});
