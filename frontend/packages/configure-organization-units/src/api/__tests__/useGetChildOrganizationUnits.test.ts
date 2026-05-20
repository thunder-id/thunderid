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
import type {OrganizationUnitListResponse} from '../../models/responses';
import useGetChildOrganizationUnits from '../useGetChildOrganizationUnits';

// Mock useThunderID
const mockHttpRequest = vi.fn();
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

describe('useGetChildOrganizationUnits', () => {
  const mockChildOUList: OrganizationUnitListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'child-1', handle: 'child-one', name: 'Child One', description: 'First child', parent: 'parent-ou'},
      {id: 'child-2', handle: 'child-two', name: 'Child Two', description: 'Second child', parent: 'parent-ou'},
    ],
    links: [{rel: 'self', href: 'https://localhost:8090/organization-units/parent-ou/ous'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch child organization units on mount', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    const {result} = renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    await waitFor(() => {
      expect(result.current.data).toEqual(mockChildOUList);
      expect(result.current.error).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: expect.stringContaining('/organization-units/parent-ou/ous') as unknown,
        method: 'GET',
      }),
    );
  });

  it('should not fetch when parentId is undefined', async () => {
    const {result} = renderHook(() => useGetChildOrganizationUnits(undefined));

    // Wait a bit to ensure query doesn't execute
    await new Promise((resolve) => {
      setTimeout(resolve, 100);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.data).toBeUndefined();
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch with default pagination params', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringMatching(/limit=30.*offset=0|offset=0.*limit=30/) as unknown,
        }),
      );
    });
  });

  it('should fetch with custom limit parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    renderHook(() => useGetChildOrganizationUnits('parent-ou', {limit: 10}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('limit=10') as unknown,
        }),
      );
    });
  });

  it('should fetch with custom offset parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    renderHook(() => useGetChildOrganizationUnits('parent-ou', {offset: 20}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('offset=20') as unknown,
        }),
      );
    });
  });

  it('should fetch with both limit and offset parameters', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    renderHook(() => useGetChildOrganizationUnits('parent-ou', {limit: 15, offset: 30}));

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

    const {result, unmount} = renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    expect(result.current.isLoading).toBe(true);

    unmount();
  });

  it('should handle API error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Failed to fetch child organization units'));

    const {result} = renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
      expect(result.current.data).toBeUndefined();
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('should return empty list when no children exist', async () => {
    const emptyList: OrganizationUnitListResponse = {
      totalResults: 0,
      startIndex: 1,
      count: 0,
      organizationUnits: [],
    };
    mockHttpRequest.mockResolvedValue({data: emptyList});

    const {result} = renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    await waitFor(() => {
      expect(result.current.data?.organizationUnits).toHaveLength(0);
      expect(result.current.data?.totalResults).toBe(0);
    });
  });

  it('should refetch when refetch is called', async () => {
    mockHttpRequest.mockResolvedValue({data: mockChildOUList});

    const {result} = renderHook(() => useGetChildOrganizationUnits('parent-ou'));

    await waitFor(() => {
      expect(result.current.data).toEqual(mockChildOUList);
    });

    const updatedList = {...mockChildOUList, totalResults: 3};
    mockHttpRequest.mockResolvedValue({data: updatedList});
    const callsBeforeRefetch = mockHttpRequest.mock.calls.length;

    await result.current.refetch();

    await waitFor(() => {
      expect(mockHttpRequest.mock.calls.length).toBeGreaterThan(callsBeforeRefetch);
    });
  });
});
