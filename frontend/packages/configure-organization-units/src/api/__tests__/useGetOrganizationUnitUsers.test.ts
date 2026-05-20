/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {OrganizationUnitUserListResponse} from '../../models/responses';
import useGetOrganizationUnitUsers from '../useGetOrganizationUnitUsers';

// Mock useThunderID
const mockHttpRequest =
  vi.fn<(requestConfig: {method: string; url: string}) => Promise<{data: OrganizationUnitUserListResponse}>>();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

// Mock useConfig
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: () => 'https://localhost:8090',
    }),
  };
});

describe('useGetOrganizationUnitUsers', () => {
  const mockUserList: OrganizationUnitUserListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    users: [
      {id: 'user-1', ouId: 'ou-123', type: 'employee'},
      {id: 'user-2', ouId: 'ou-123', type: 'contractor'},
    ],
    links: [{rel: 'self', href: 'https://localhost:8090/organization-units/ou-123/users'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch organization unit users on mount', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    const {result} = renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    await waitFor(() => {
      expect(result.current.data).toEqual(mockUserList);
      expect(result.current.error).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: expect.stringContaining('/organization-units/ou-123/users') as unknown,
        method: 'GET',
      }),
    );
  });

  it('should not fetch when organizationUnitId is undefined', async () => {
    const {result} = renderHook(() => useGetOrganizationUnitUsers(undefined));

    // Wait a bit to ensure query doesn't execute
    await new Promise((resolve) => {
      setTimeout(resolve, 100);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.data).toBeUndefined();
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch with default pagination params', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringMatching(/limit=30.*offset=0|offset=0.*limit=30/) as unknown,
        }),
      );
    });
  });

  it('should fetch with custom limit parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    renderHook(() => useGetOrganizationUnitUsers('ou-123', {limit: 10}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('limit=10') as unknown,
        }),
      );
    });
  });

  it('should fetch with custom offset parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    renderHook(() => useGetOrganizationUnitUsers('ou-123', {offset: 20}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('offset=20') as unknown,
        }),
      );
    });
  });

  it('should fetch with both limit and offset parameters', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    renderHook(() => useGetOrganizationUnitUsers('ou-123', {limit: 15, offset: 30}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringMatching(/limit=15.*offset=30|offset=30.*limit=15/) as unknown,
        }),
      );
    });
  });

  it('should set loading state during fetch', () => {
    mockHttpRequest.mockImplementation(
      () =>
        new Promise(() => {
          // Never resolve to keep loading state
        }),
    );

    const {result, unmount} = renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    expect(result.current.isLoading).toBe(true);

    unmount();
  });

  it('should handle API error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Failed to fetch users'));

    const {result} = renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
      expect(result.current.data).toBeUndefined();
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('should return empty list when no users exist', async () => {
    const emptyList: OrganizationUnitUserListResponse = {
      totalResults: 0,
      startIndex: 1,
      count: 0,
      users: [],
    };
    mockHttpRequest.mockResolvedValue({data: emptyList});

    const {result} = renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    await waitFor(() => {
      expect(result.current.data?.users).toHaveLength(0);
      expect(result.current.data?.totalResults).toBe(0);
    });
  });

  it('should refetch when refetch is called', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUserList});

    const {result} = renderHook(() => useGetOrganizationUnitUsers('ou-123'));

    await waitFor(() => {
      expect(result.current.data).toEqual(mockUserList);
    });

    const updatedList = {...mockUserList, totalResults: 3};
    mockHttpRequest.mockResolvedValue({data: updatedList});
    const callsBeforeRefetch = mockHttpRequest.mock.calls.length;

    await result.current.refetch();

    await waitFor(() => {
      expect(mockHttpRequest.mock.calls.length).toBeGreaterThan(callsBeforeRefetch);
    });
  });
});
