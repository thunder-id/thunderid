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
import UserQueryKeys from '../../constants/user-query-keys';
import type {UserListResponse} from '../../models/users';
import useGetUsers from '../useGetUsers';

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

describe('useGetUsers', () => {
  const mockUserListResponse: UserListResponse = {
    totalResults: 2,
    startIndex: 0,
    count: 2,
    users: [
      {id: 'user-1', ouId: 'ou-1', type: 'Employee', attributes: {username: 'john'}},
      {id: 'user-2', ouId: 'ou-1', type: 'Employee', attributes: {username: 'jane'}},
    ],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetUsers());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch users list', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    const {result} = renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUserListResponse);
    expect(result.current.data?.users).toHaveLength(2);
    expect(result.current.data?.totalResults).toBe(2);
    expect(result.current.data?.count).toBe(2);
  });

  it('should fetch users without query parameters when no params provided', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toBe('https://api.test.com/users?include=display');
  });

  it('should use custom pagination parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers({limit: 10, offset: 5}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('limit=10');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('offset=5');
  });

  it('should use filter parameter', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers({filter: 'name eq "John"'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    const url = callArgs.url as string;
    expect(url).toContain('filter=');
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch users');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/users');
  });

  it('should include correct headers', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.method).toBe('GET');
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });

  it('should use correct query key with pagination params', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    const {result, queryClient} = renderHook(() => useGetUsers({limit: 20, offset: 10}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cache = queryClient.getQueryCache();
    const queries = cache.findAll({
      queryKey: [UserQueryKeys.USERS, {limit: 20, offset: 10, filter: undefined}],
    });

    expect(queries).toHaveLength(1);
  });

  it('should cache results for same parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetUsers({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([UserQueryKeys.USERS, {limit: 10, offset: 0, filter: undefined}], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetUsers({limit: 10, offset: 0}), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.data).toEqual(mockUserListResponse);
    });
    expect(mockHttpRequest).toHaveBeenCalledTimes(1); // Should not make another request
  });

  it('should make new request for different parameters', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockUserListResponse,
    });

    const {result: result1} = renderHook(() => useGetUsers({limit: 10, offset: 0}));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Second call with different parameters should make new request
    const {result: result2} = renderHook(() => useGetUsers({limit: 20, offset: 5}));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockUserListResponse,
    });

    const {result} = renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Refetch the data
    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle empty users list', async () => {
    const emptyResponse: UserListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      users: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetUsers());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyResponse);
    expect(result.current.data?.users).toHaveLength(0);
    expect(result.current.data?.totalResults).toBe(0);
  });

  it('should properly construct query string with all parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUserListResponse,
    });

    renderHook(() => useGetUsers({limit: 15, offset: 30, filter: 'type eq "admin"'}));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    const url = callArgs.url as string;
    expect(url).toContain('users?');
    expect(url).toContain('limit=15');
    expect(url).toContain('offset=30');
    expect(url).toContain('filter=');
  });

  it('should maintain correct loading state during fetch', async () => {
    let resolveRequest: (value: {data: UserListResponse}) => void;
    const requestPromise = new Promise<{data: UserListResponse}>((resolve) => {
      resolveRequest = resolve;
    });

    mockHttpRequest.mockReturnValueOnce(requestPromise);

    const {result} = renderHook(() => useGetUsers());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();

    resolveRequest!({data: mockUserListResponse});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toEqual(mockUserListResponse);
  });
});
