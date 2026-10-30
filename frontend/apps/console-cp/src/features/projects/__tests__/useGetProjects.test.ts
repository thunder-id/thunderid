// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {ProjectListResponse} from '../useGetProjects';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://localhost:8090'}),
  };
});

const {default: useGetProjects} = await import('../useGetProjects');

describe('useGetProjects', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('reads the projects from the project API', async () => {
    const projects: ProjectListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      projects: [
        {
          id: 'p1',
          handle: 'shop',
          displayName: 'Shop',
          rootOuId: 'ou1',
          createdAt: '2026-10-01T00:00:00Z',
        },
      ],
    };
    mockHttpRequest.mockResolvedValue({data: projects});

    const {result} = renderHook(() => useGetProjects());

    await waitFor(() => expect(result.current.data).toEqual(projects));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/projects', method: 'GET'}),
    );
  });

  it('does not retry a failed read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Forbidden'));

    const {result} = renderHook(() => useGetProjects());

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });
});
