// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {GroupListResponse} from '../../models/group';

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

const {default: useGetGroups} = await import('../useGetGroups');

describe('useGetGroups', () => {
  const mockGroupsData: GroupListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    groups: [
      {id: 'g1', name: 'Group One', ouId: 'ou1'},
      {id: 'g2', name: 'Group Two', description: 'Desc', ouId: 'ou2'},
    ],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch groups with default pagination', async () => {
    mockHttpRequest.mockResolvedValue({data: mockGroupsData});
    const {result} = renderHook(() => useGetGroups());

    await waitFor(() => {
      expect(result.current.data).toEqual(mockGroupsData);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups?limit=30&offset=0&include=display',
        method: 'GET',
      }),
    );
  });

  it('should fetch groups with custom pagination', async () => {
    mockHttpRequest.mockResolvedValue({data: mockGroupsData});
    renderHook(() => useGetGroups({limit: 10, offset: 20}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/groups?limit=10&offset=20&include=display',
        }),
      );
    });
  });

  it('should pass the filter to the server', async () => {
    mockHttpRequest.mockResolvedValue({data: mockGroupsData});
    renderHook(() => useGetGroups({limit: 10, offset: 0, filter: 'ouId eq "ou-1"'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/groups?limit=10&offset=0&include=display&filter=ouId+eq+%22ou-1%22',
        }),
      );
    });
  });

  it('should set loading state', () => {
    mockHttpRequest.mockImplementation(() => new Promise(() => null));
    const {result, unmount} = renderHook(() => useGetGroups());

    expect(result.current.isLoading).toBe(true);
    unmount();
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Network error'));
    const {result} = renderHook(() => useGetGroups());

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
      expect(result.current.error?.message).toBe('Network error');
    });
  });

  it('should keep the previous page data while fetching a new page', async () => {
    const nextPageData: GroupListResponse = {
      totalResults: 2,
      startIndex: 11,
      count: 2,
      groups: [
        {id: 'g3', name: 'Group Three', ouId: 'ou1'},
        {id: 'g4', name: 'Group Four', ouId: 'ou2'},
      ],
    };
    let resolveNextPage: ((value: {data: GroupListResponse}) => void) | undefined;
    mockHttpRequest.mockImplementation((request: {url: string}) => {
      if (request.url.includes('offset=0')) {
        return Promise.resolve({data: mockGroupsData});
      }

      if (request.url.includes('offset=10')) {
        return new Promise((resolve) => {
          resolveNextPage = resolve;
        });
      }

      throw new Error(`Unexpected groups request: ${request.url}`);
    });

    const {result, rerender} = renderHook(({offset}: {offset: number}) => useGetGroups({limit: 10, offset}), {
      initialProps: {offset: 0},
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(mockGroupsData);
    });

    rerender({offset: 10});

    expect(result.current.data).toEqual(mockGroupsData);
    expect(result.current.isFetching).toBe(true);
    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/groups?limit=10&offset=10&include=display',
        }),
      );
      expect(resolveNextPage).toBeDefined();
    });

    if (!resolveNextPage) {
      throw new Error('The next-page request was not captured');
    }
    resolveNextPage({data: nextPageData});

    await waitFor(() => {
      expect(result.current.isFetching).toBe(false);
      expect(result.current.data).toEqual(nextPageData);
    });
  });

  it('should not show the previous filter results while fetching another filter', async () => {
    const otherOUData: GroupListResponse = {
      totalResults: 1,
      startIndex: 1,
      count: 1,
      groups: [{id: 'g5', name: 'Group Five', ouId: 'ou-2'}],
    };
    let resolveOtherOU: ((value: {data: GroupListResponse}) => void) | undefined;
    mockHttpRequest.mockImplementation((request: {url: string}) => {
      if (request.url.includes('ou-1')) {
        return Promise.resolve({data: mockGroupsData});
      }

      if (request.url.includes('ou-2')) {
        return new Promise((resolve) => {
          resolveOtherOU = resolve;
        });
      }

      throw new Error(`Unexpected groups request: ${request.url}`);
    });

    const {result, rerender} = renderHook(
      ({filter}: {filter: string}) => useGetGroups({limit: 10, offset: 0, filter}),
      {
        initialProps: {filter: 'ouId eq "ou-1"'},
      },
    );

    await waitFor(() => {
      expect(result.current.data).toEqual(mockGroupsData);
    });

    rerender({filter: 'ouId eq "ou-2"'});

    expect(result.current.data).toBeUndefined();
    expect(result.current.isLoading).toBe(true);
    await waitFor(() => {
      expect(resolveOtherOU).toBeDefined();
    });

    if (!resolveOtherOU) {
      throw new Error('The other organization unit request was not captured');
    }
    resolveOtherOU({data: otherOUData});

    await waitFor(() => {
      expect(result.current.data).toEqual(otherOUData);
    });
  });
});
