// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import ProjectQueryKeys from '../../constants/project-query-keys';
import type {Project, ProjectListResponse} from '../../models/project';
import useCreateProject from '../useCreateProject';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  useToast: () => ({showToast: vi.fn()}),
}));

const existing: Project = {
  id: 'p1',
  handle: 'shop',
  displayName: 'Shop',
  createdAt: '2026-10-01T00:00:00Z',
};

const created: Project = {
  id: 'p2',
  handle: 'support',
  displayName: 'Support',
  createdAt: '2026-10-02T00:00:00Z',
};

describe('useCreateProject', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('adds the created project to the project list straight away', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: created}).mockReturnValue(new Promise(() => null));
    const {result, queryClient} = renderHook(() => useCreateProject());
    queryClient.setQueryData<ProjectListResponse>([ProjectQueryKeys.PROJECTS], {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      projects: [existing],
    });

    result.current.mutate({handle: 'support', displayName: 'Support'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryData<ProjectListResponse>([ProjectQueryKeys.PROJECTS])).toEqual({
      totalResults: 2,
      startIndex: 1,
      count: 2,
      projects: [existing, created],
    });
  });

  it('does not add the project twice', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: created}).mockReturnValue(new Promise(() => null));
    const {result, queryClient} = renderHook(() => useCreateProject());
    const list: ProjectListResponse = {totalResults: 1, startIndex: 1, count: 1, projects: [created]};
    queryClient.setQueryData<ProjectListResponse>([ProjectQueryKeys.PROJECTS], list);

    result.current.mutate({handle: 'support', displayName: 'Support'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(queryClient.getQueryData<ProjectListResponse>([ProjectQueryKeys.PROJECTS])).toEqual(list);
  });
});
