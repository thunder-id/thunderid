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

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import type {User} from '@thunderid/types';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserQueryKeys from '../../constants/user-query-keys';
import useUpdateUser, {type UpdateUserVariables} from '../useUpdateUser';

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

describe('useUpdateUser', () => {
  const mockUser: User = {
    id: 'user-1',
    ouId: 'ou-1',
    type: 'Employee',
    attributes: {username: 'john-updated', email: 'john@test.com'},
  };

  const mockVariables: UpdateUserVariables = {
    userId: 'user-1',
    data: {
      ouId: 'ou-1',
      type: 'Employee',
      attributes: {username: 'john-updated', email: 'john@test.com'},
    },
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useUpdateUser());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully update a user', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    act(() => {
      result.current.mutate(mockVariables);
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUser);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with user ID and data', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/users/${mockVariables.userId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockVariables.data,
      }),
    );
  });

  it('should set pending state during update', async () => {
    let resolveRequest!: (value: {data: User}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    // Now resolve the request
    act(() => {
      resolveRequest({data: mockUser});
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to update user');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

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

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate user and users queries on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result, queryClient} = renderHook(() => useUpdateUser());

    // Pre-populate cache with original user
    const originalUser = {...mockUser, attributes: {username: 'john', email: 'john@test.com'}};
    queryClient.setQueryData([UserQueryKeys.USER, mockVariables.userId], originalUser);
    queryClient.setQueryData([UserQueryKeys.USERS], {
      users: [originalUser],
      totalResults: 1,
      count: 1,
    });

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // Verify that invalidateQueries was called for both the specific user and the list
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [UserQueryKeys.USER, mockVariables.userId],
      }),
    );
    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [UserQueryKeys.USERS],
      }),
    );
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result, queryClient} = renderHook(() => useUpdateUser());

    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValue(new Error('Invalidation failed'));

    result.current.mutate(mockVariables);

    // The mutation should still succeed even if invalidateQueries fails
    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUser);
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    const promise = result.current.mutateAsync(mockVariables);

    await expect(promise).resolves.toEqual(mockUser);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(mockUser);
  });

  it('should reject mutateAsync on error', async () => {
    const apiError = new Error('Update failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateUser());

    const promise = result.current.mutateAsync(mockVariables);

    await expect(promise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.data).toBeUndefined();
    });
    expect(result.current.error).toBeNull();
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
  });

  it('should handle multiple sequential updates', async () => {
    const user1 = {...mockUser, attributes: {username: 'update-1', email: 'john@test.com'}};
    const user2 = {...mockUser, attributes: {username: 'update-2', email: 'john@test.com'}};

    mockHttpRequest.mockResolvedValueOnce({data: user1}).mockResolvedValueOnce({data: user2});

    const {result} = renderHook(() => useUpdateUser());

    // First update
    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(user1);

    // Second update
    result.current.mutate({
      ...mockVariables,
      data: {...mockVariables.data, attributes: {username: 'update-2', email: 'john@test.com'}},
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(user2);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should update different users independently', async () => {
    const user1 = {...mockUser, id: 'user-1', attributes: {username: 'user1-updated', email: 'user1@test.com'}};
    const user2 = {...mockUser, id: 'user-2', attributes: {username: 'user2-updated', email: 'user2@test.com'}};

    mockHttpRequest.mockResolvedValueOnce({data: user1}).mockResolvedValueOnce({data: user2});

    const {result} = renderHook(() => useUpdateUser());

    // Update first user
    result.current.mutate({
      userId: 'user-1',
      data: mockVariables.data,
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('user-1');

    // Update second user
    result.current.mutate({
      userId: 'user-2',
      data: mockVariables.data,
    });

    await waitFor(() => {
      expect(result.current.data?.id).toBe('user-2');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    mockGetServerUrl.mockReturnValue(customServerUrl);

    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${customServerUrl}/users/${mockVariables.userId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockVariables.data,
      }),
    );
  });

  it('should properly serialize request data as JSON', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useUpdateUser());

    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.data).toEqual(mockVariables.data);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce({data: mockUser});

    const {result} = renderHook(() => useUpdateUser());

    // First attempt - should fail
    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);

    // Second attempt - should succeed
    result.current.mutate(mockVariables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });
});
