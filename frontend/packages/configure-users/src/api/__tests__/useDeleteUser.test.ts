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
import useDeleteUser from '../useDeleteUser';

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

describe('useDeleteUser', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteUser());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully delete a user', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with user ID', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/users/${userId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should set pending state during deletion', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve(undefined), 100);
      }),
    );

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    await waitFor(
      () => {
        expect(result.current.isSuccess).toBe(true);
      },
      {timeout: 200},
    );

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete user');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should handle network error', async () => {
    const networkError = new Error('Network request failed');
    mockHttpRequest.mockRejectedValueOnce(networkError);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.isPending).toBe(false);
  });

  it('should remove user from cache on successful deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result, queryClient} = renderHook(() => useDeleteUser());

    // Pre-populate cache with user
    queryClient.setQueryData([UserQueryKeys.USER, userId], {
      id: userId,
      ouId: 'ou-1',
      type: 'Employee',
      attributes: {username: 'john'},
    });

    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that removeQueries was called for the specific user
    expect(removeQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [UserQueryKeys.USER, userId],
      }),
    );
  });

  it('should invalidate users list on successful deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result, queryClient} = renderHook(() => useDeleteUser());

    // Pre-populate cache with users list
    queryClient.setQueryData([UserQueryKeys.USERS], {
      users: [
        {
          id: userId,
          ouId: 'ou-1',
          type: 'Employee',
          attributes: {username: 'john'},
        },
      ],
      totalResults: 1,
      count: 1,
    });

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that invalidateQueries was called for the users list
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [UserQueryKeys.USERS],
      }),
    );
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result, queryClient} = renderHook(() => useDeleteUser());

    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(userId);

    // The mutation should still succeed even if invalidateQueries fails
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should handle sequential deletions', async () => {
    mockHttpRequest.mockResolvedValue(undefined);

    const user1Id = 'user-1';
    const user2Id = 'user-2';

    const {result} = renderHook(() => useDeleteUser());

    // Delete first user
    result.current.mutate(user1Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Delete second user
    result.current.mutate(user2Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: `https://api.test.com/users/${user1Id}`,
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: `https://api.test.com/users/${user2Id}`,
      }),
    );
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    const deletePromise = result.current.mutateAsync(userId);

    await expect(deletePromise).resolves.toBeUndefined();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should reject mutateAsync on error', async () => {
    const apiError = new Error('Deletion failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    const deletePromise = result.current.mutateAsync(userId);

    await expect(deletePromise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    // First attempt - should fail
    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);

    // Second attempt - should succeed
    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should not affect other cached users on deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const user1Id = 'user-1';
    const user2Id = 'user-2';

    // Pre-populate cache with two users
    const user1Data = {id: user1Id, ouId: 'ou-1', type: 'Employee'};
    const user2Data = {id: user2Id, ouId: 'ou-1', type: 'Employee'};

    const {result, queryClient} = renderHook(() => useDeleteUser());

    queryClient.setQueryData([UserQueryKeys.USER, user1Id], user1Data);
    queryClient.setQueryData([UserQueryKeys.USER, user2Id], user2Data);

    // Delete first user
    result.current.mutate(user1Id);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that user2 is still in the cache
    const user2InCache = queryClient.getQueryData([UserQueryKeys.USER, user2Id]);
    expect(user2InCache).toEqual(user2Data);
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    mockGetServerUrl.mockReturnValue(customServerUrl);

    mockHttpRequest.mockResolvedValueOnce(undefined);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${customServerUrl}/users/${userId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should pass through server error messages', async () => {
    const serverError = new Error('User has active sessions and cannot be deleted');
    mockHttpRequest.mockRejectedValueOnce(serverError);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(serverError);
    expect(result.current.error?.message).toBe('User has active sessions and cannot be deleted');
  });
});
