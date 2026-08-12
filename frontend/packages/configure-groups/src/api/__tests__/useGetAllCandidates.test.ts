// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

const {default: useGetAllCandidates} = await import('../useGetAllCandidates');

describe('useGetAllCandidates', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches later pages so candidates are not lost after filtering', async () => {
    mockHttpRequest
      .mockResolvedValueOnce({
        data: {
          totalResults: 101,
          startIndex: 1,
          count: 100,
          users: Array.from({length: 100}, (_, index) => ({id: `existing-${index}`})),
        },
      })
      .mockResolvedValueOnce({
        data: {
          totalResults: 101,
          startIndex: 101,
          count: 1,
          users: [{id: 'available-user'}],
        },
      });

    const {result} = renderHook(() =>
      useGetAllCandidates<{id: string}>({path: '/users', itemsKey: 'users', enabled: true}),
    );

    await waitFor(() => {
      expect(result.current.data).toHaveLength(101);
    });

    expect(result.current.data).toContainEqual({id: 'available-user'});
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: 'https://localhost:8090/users?limit=100&offset=100&include=display',
      }),
    );
  });
});
