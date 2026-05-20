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

/* eslint-disable @typescript-eslint/no-unsafe-assignment */
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {OrganizationUnitListResponse} from '../../models/responses';
import useGetOrganizationUnits from '../useGetOrganizationUnits';

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

describe('useGetOrganizationUnits', () => {
  const mockOrganizationUnitList: OrganizationUnitListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    organizationUnits: [
      {id: 'ou-1', handle: 'root', name: 'Root Organization', description: 'Root OU', parent: null},
      {id: 'ou-2', handle: 'child', name: 'Child Organization', description: 'Child OU', parent: 'ou-1'},
    ],
    links: [{rel: 'self', href: 'https://localhost:8090/organization-units'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch organization units on mount', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    const {result} = renderHook(() => useGetOrganizationUnits());

    await waitFor(() => {
      expect(result.current.data).toEqual(mockOrganizationUnitList);
      expect(result.current.error).toBeNull();
      expect(result.current.isLoading).toBe(false);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: expect.stringContaining('/organization-units'),
        method: 'GET',
      }),
    );
  });

  it('should fetch organization units with limit parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({limit: 10}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('limit=10'),
          method: 'GET',
        }),
      );
    });
  });

  it('should fetch organization units with offset parameter', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({offset: 5}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('offset=5'),
          method: 'GET',
        }),
      );
    });
  });

  it('should fetch organization units with both limit and offset parameters', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({limit: 10, offset: 5}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringMatching(/limit=10.*offset=5|offset=5.*limit=10/),
          method: 'GET',
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

    const {result, unmount} = renderHook(() => useGetOrganizationUnits());

    expect(result.current.isLoading).toBe(true);

    unmount();
  });

  it('should handle API error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Failed to fetch organization units'));

    const {result} = renderHook(() => useGetOrganizationUnits());

    await waitFor(() => {
      expect(result.current.error).not.toBeNull();
      expect(result.current.data).toBeUndefined();
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('should refetch when refetch is called', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    const {result} = renderHook(() => useGetOrganizationUnits());

    await waitFor(() => {
      expect(result.current.data).toEqual(mockOrganizationUnitList);
    });

    const updatedList = {...mockOrganizationUnitList, totalResults: 3};
    mockHttpRequest.mockResolvedValue({data: updatedList});
    const callsBeforeRefetch = mockHttpRequest.mock.calls.length;

    await result.current.refetch();

    await waitFor(() => {
      expect(mockHttpRequest.mock.calls.length).toBeGreaterThan(callsBeforeRefetch);
      expect(result.current.data).toEqual(updatedList);
    });
  });

  it('should use default params when no params provided', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('/organization-units'),
          method: 'GET',
        }),
      );
    });
  });

  it('should use default values when params object is empty', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('limit=30'),
        }),
      );
    });
  });

  it('should use default offset when only limit provided', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({limit: 50}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('offset=0'),
        }),
      );
    });
  });

  it('should use default limit when only offset provided', async () => {
    mockHttpRequest.mockResolvedValue({data: mockOrganizationUnitList});

    renderHook(() => useGetOrganizationUnits({offset: 10}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: expect.stringContaining('limit=30'),
        }),
      );
    });
  });
});
