// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {Project} from '../useGetProjects';
import useProjectSelection, {PROJECT_SELECTION_STORAGE_KEY} from '../useProjectSelection';

const mockUseGetProjects = vi.fn<() => unknown>();

vi.mock('../useGetProjects', () => ({
  default: () => mockUseGetProjects(),
}));

function project(id: string): Project {
  return {id, handle: id, displayName: id.toUpperCase(), rootOuId: `ou-${id}`, createdAt: '2026-10-01T00:00:00Z'};
}

describe('useProjectSelection', () => {
  beforeEach(() => {
    localStorage.clear();
    mockUseGetProjects.mockReturnValue({
      data: {projects: [project('shop'), project('support')]},
      isLoading: false,
      error: null,
      refetch: vi.fn(),
    });
  });

  it('selects the first project until one is chosen', () => {
    const {result} = renderHook(() => useProjectSelection());

    expect(result.current.selectedProject?.id).toBe('shop');
    expect(result.current.isCleared).toBe(false);
  });

  it('selects the chosen project and remembers it', () => {
    const {result} = renderHook(() => useProjectSelection());

    act(() => result.current.selectProject('support'));

    expect(result.current.selectedProject?.id).toBe('support');
    expect(localStorage.getItem(PROJECT_SELECTION_STORAGE_KEY)).toBe('support');
  });

  it('has no selected project once the selection is cleared', () => {
    const {result} = renderHook(() => useProjectSelection());

    act(() => result.current.clearSelection());

    expect(result.current.selectedProject).toBeUndefined();
    expect(result.current.isCleared).toBe(true);
  });

  it('shares the selection between every reader', () => {
    const first = renderHook(() => useProjectSelection());
    const second = renderHook(() => useProjectSelection());

    act(() => first.result.current.selectProject('support'));

    expect(second.result.current.selectedProject?.id).toBe('support');
  });

  it('falls back to the first project when the remembered one no longer exists', () => {
    localStorage.setItem(PROJECT_SELECTION_STORAGE_KEY, 'removed');

    const {result} = renderHook(() => useProjectSelection());

    expect(result.current.selectedProject?.id).toBe('shop');
  });
});
