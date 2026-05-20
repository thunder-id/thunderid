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
import type {CreateUserRequest} from '../../models/users';
import useCreateUser from '../useCreateUser';

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

describe('useCreateUser', () => {
  const mockUser: User = {
    id: 'user-1',
    ouId: 'ou-1',
    type: 'Employee',
    attributes: {username: 'john', email: 'john@test.com'},
  };

  const mockRequest: CreateUserRequest = {
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

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useCreateUser());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully create a user', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUser);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/users',
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockRequest,
      }),
    );
  });

  it('should set pending state during creation', async () => {
    let resolveRequest: (value: unknown) => void;
    const requestPromise = new Promise((resolve) => {
      resolveRequest = resolve;
    });
    mockHttpRequest.mockReturnValue(requestPromise);

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest!({data: mockUser});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create user');

    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

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

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate users query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result, queryClient} = renderHook(() => useCreateUser());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

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

    const {result, queryClient} = renderHook(() => useCreateUser());
    // Mock invalidateQueries to reject
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(mockRequest);

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

    const {result} = renderHook(() => useCreateUser());

    const promise = result.current.mutateAsync(mockRequest);

    await expect(promise).resolves.toEqual(mockUser);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
    expect(result.current.data).toEqual(mockUser);
  });

  it('should reject mutateAsync on error', async () => {
    const apiError = new Error('Creation failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateUser());

    const promise = result.current.mutateAsync(mockRequest);

    await expect(promise).rejects.toEqual(apiError);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

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

  it('should handle multiple sequential mutations', async () => {
    const firstUser = {...mockUser, id: 'first-id'};
    const secondUser = {...mockUser, id: 'second-id'};

    mockHttpRequest.mockResolvedValueOnce({
      data: firstUser,
    });

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
      expect(result.current.data).toEqual(firstUser);
    });

    mockHttpRequest.mockResolvedValueOnce({
      data: secondUser,
    });

    result.current.mutate({...mockRequest, type: 'Contractor'});

    await waitFor(() => {
      expect(result.current.data).toEqual(secondUser);
    });
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    mockGetServerUrl.mockReturnValue(customServerUrl);

    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${customServerUrl}/users`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: mockRequest,
      }),
    );
  });

  it('should properly serialize request data as JSON', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUser,
    });

    const {result} = renderHook(() => useCreateUser());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.data).toEqual(mockRequest);
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce({data: mockUser});

    const {result} = renderHook(() => useCreateUser());

    // First attempt - should fail
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);

    // Second attempt - should succeed
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });
});
