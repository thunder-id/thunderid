/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
});
