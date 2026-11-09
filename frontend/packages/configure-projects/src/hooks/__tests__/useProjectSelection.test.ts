// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../../models/project';
import useProjectSelection from '../useProjectSelection';

const mockUseGetProjects = vi.fn<() => unknown>();
const mockNavigate = vi.fn();
let mockParams: Record<string, string> = {};

vi.mock('../../api/useGetProjects', () => ({
  default: () => mockUseGetProjects(),
}));

vi.mock('react-router', async (importOriginal) => ({
  ...(await importOriginal<typeof import('react-router')>()),
  useNavigate: () => mockNavigate,
  useParams: () => mockParams,
}));

function project(id: string, handle: string): Project {
  return {id, handle, displayName: handle.toUpperCase(), createdAt: '2026-10-01T00:00:00Z'};
}

describe('useProjectSelection', () => {
  beforeEach(() => {
    mockNavigate.mockReset();
    mockParams = {};
    mockUseGetProjects.mockReturnValue({
      data: {projects: [project('p1', 'shop'), project('p2', 'support')]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('selects the project the route names by its handle', () => {
    mockParams = {projectHandle: 'support'};

    const {result} = renderHook(() => useProjectSelection());

    expect(result.current.selectedHandle).toBe('support');
    expect(result.current.selectedProject?.id).toBe('p2');
  });

  it('selects no project on a page outside every project', () => {
    const {result} = renderHook(() => useProjectSelection());

    expect(result.current.selectedHandle).toBeUndefined();
    expect(result.current.selectedProject).toBeUndefined();
  });

  it('selects no project when no project has the handle the route names', () => {
    mockParams = {projectHandle: 'removed'};

    const {result} = renderHook(() => useProjectSelection());

    expect(result.current.selectedHandle).toBe('removed');
    expect(result.current.selectedProject).toBeUndefined();
  });

  it("opens a project's home, or a page within it", () => {
    const {result} = renderHook(() => useProjectSelection());

    result.current.selectProject('support');
    result.current.selectProject('my shop', '/applications');

    expect(mockNavigate).toHaveBeenNthCalledWith(1, '/project/support');
    expect(mockNavigate).toHaveBeenNthCalledWith(2, '/project/my%20shop/applications');
  });
});
