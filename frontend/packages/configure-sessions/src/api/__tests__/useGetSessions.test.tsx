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

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import SessionQueryKeys from '../../constants/session-query-keys';
import type {SessionListFilter, SessionListResponse} from '../../models/sessions';
import useGetSessions from '../useGetSessions';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: mockGetServerUrl,
    }),
  };
});

describe('useGetSessions', () => {
  const mockSessionListResponse: SessionListResponse = {
    totalResults: 1,
    startIndex: 1,
    count: 1,
    sessions: [
      {
        id: 'session-1',
        userId: 'u1',
        loginFlowId: 'flow-1',
        authenticatedAt: '2026-01-01T00:00:00Z',
        createdAt: '2026-01-01T00:00:00Z',
        lastActiveAt: '2026-01-01T00:00:00Z',
        participants: [{appId: 'a1', firstJoinedAt: '2026-01-01T00:00:00Z', lastActiveAt: '2026-01-01T00:00:00Z'}],
      },
    ],
    links: [],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should build the correct URL for a userId filter with pagination params', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockSessionListResponse});

    renderHook(() => useGetSessions({userId: 'u1'}, {limit: 10, offset: 0}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/sessions?userId=u1&limit=10&offset=0',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should build the correct URL for an appId filter without params', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockSessionListResponse});

    renderHook(() => useGetSessions({appId: 'a1'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/sessions?appId=a1',
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should return response.data', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockSessionListResponse});

    const {result} = renderHook(() => useGetSessions({userId: 'u1'}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockSessionListResponse);
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockSessionListResponse});

    const {result, queryClient} = renderHook(() => useGetSessions({userId: 'u1'}, {limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [SessionQueryKeys.SESSIONS, {userId: 'u1', appId: undefined, limit: 10, offset: 0}];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockSessionListResponse);
  });

  it('should not make API call when the filter is empty', () => {
    const {result} = renderHook(() => useGetSessions({} as SessionListFilter));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not make API call when userId is empty string', () => {
    const {result} = renderHook(() => useGetSessions({userId: ''}));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch sessions');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetSessions({userId: 'u1'}));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should refetch when the filter changes', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockSessionListResponse}).mockResolvedValueOnce({
      data: {...mockSessionListResponse, totalResults: 2},
    });

    const {result, rerender} = renderHook(({userId}: {userId: string}) => useGetSessions({userId}), {
      initialProps: {userId: 'u1'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    rerender({userId: 'u2'});

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    });
  });
});
