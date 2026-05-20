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
import type {User} from '@thunderid/types';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserQueryKeys from '../../constants/user-query-keys';
import useGetUser from '../useGetUser';

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

describe('useGetUser', () => {
  const mockUser: User = {
    id: 'user-1',
    ouId: 'ou-1',
    type: 'Employee',
    attributes: {username: 'john', email: 'john@test.com'},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state when userId is provided', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null)); // Never resolves

    const {result} = renderHook(() => useGetUser('user-1'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch a single user', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const userId = 'user-1';
    const {result} = renderHook(() => useGetUser(userId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUser);
    expect(result.current.data?.id).toBe(userId);
    expect(result.current.data?.attributes?.['username']).toBe('john');
  });

  it('should make correct API call with user ID', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const userId = 'user-1';
    renderHook(() => useGetUser(userId));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/users/${userId}?include=display`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const userId = 'user-1';
    const {result, queryClient} = renderHook(() => useGetUser(userId));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const queryKey = [UserQueryKeys.USER, userId];
    const cachedData = queryClient.getQueryData(queryKey);
    expect(cachedData).toEqual(mockUser);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch user');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetUser('user-1'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const {result} = renderHook(() => useGetUser('user-1'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
  });

  it('should not make API call when userId is undefined', () => {
    const {result} = renderHook(() => useGetUser(undefined));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not make API call when userId is empty string', () => {
    const {result} = renderHook(() => useGetUser(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should use correct server URL from config', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    renderHook(() => useGetUser('user-1'));

    await waitFor(() => {
      expect(mockGetServerUrl).toHaveBeenCalledTimes(1);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.url).toContain('https://api.test.com/users/user-1');
  });

  it('should include correct headers', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    renderHook(() => useGetUser('user-1'));

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

  it('should refetch when userId changes', async () => {
    const user1 = {...mockUser, id: 'user-1', attributes: {username: 'john'}};
    const user2 = {...mockUser, id: 'user-2', attributes: {username: 'jane'}};

    mockHttpRequest.mockResolvedValueOnce({data: user1}).mockResolvedValueOnce({data: user2});

    const {result, rerender} = renderHook(({userId}: {userId: string}) => useGetUser(userId), {
      initialProps: {userId: 'user-1'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('user-1');
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Change the user ID
    rerender({userId: 'user-2'});

    await waitFor(() => {
      expect(result.current.data?.id).toBe('user-2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should cache user data', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const userId = 'user-1';

    // First call - get the queryClient from the render result
    const {result: result1, queryClient} = renderHook(() => useGetUser(userId));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Set the data as fresh to prevent refetch
    queryClient.setQueryDefaults([UserQueryKeys.USER, userId], {
      staleTime: Infinity,
    });

    // Second call with same queryClient should use cache
    const {result: result2} = renderHook(() => useGetUser(userId), {
      queryClient,
    });

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    // Should still be called only once due to caching
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(result2.current.data).toEqual(mockUser);
  });

  it('should support refetching data', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockUser,
    });

    const {result} = renderHook(() => useGetUser('user-1'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);

    // Refetch the data
    await result.current.refetch();

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should handle different user IDs', async () => {
    const user1 = {...mockUser, id: 'user-1', attributes: {username: 'john'}};
    const user2 = {...mockUser, id: 'user-2', attributes: {username: 'jane'}};

    mockHttpRequest.mockResolvedValueOnce({data: user1});

    const {result: result1} = renderHook(() => useGetUser('user-1'));

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    expect(result1.current.data?.id).toBe('user-1');

    mockHttpRequest.mockResolvedValueOnce({data: user2});

    const {result: result2} = renderHook(() => useGetUser('user-2'));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    expect(result2.current.data?.id).toBe('user-2');
  });

  it('should maintain correct loading state during fetch', async () => {
    let resolveRequest: (value: {data: User}) => void;
    const requestPromise = new Promise<{data: User}>((resolve) => {
      resolveRequest = resolve;
    });

    mockHttpRequest.mockReturnValueOnce(requestPromise);

    const {result} = renderHook(() => useGetUser('user-1'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.isFetching).toBe(true);
    expect(result.current.data).toBeUndefined();

    resolveRequest!({data: mockUser});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isLoading).toBe(false);
    expect(result.current.isFetching).toBe(false);
    expect(result.current.data).toEqual(mockUser);
  });
});
