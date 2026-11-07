// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserQueryKeys from '../../constants/user-query-keys';
import useDeleteUser from '../useDeleteUser';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
const mockShowToast = vi.fn();

// How the console says this operation runs. A console that declares nothing gets the flow mode.
let administrationOperation: unknown = AdministrationModes.FLOW;

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
    useToast: () => ({
      showToast: mockShowToast,
    }),
    useAdministrationOperation: () => administrationOperation,
  };
});

/**
 * The request shape the hook issues, as far as these tests inspect it.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * Answers the mocked client by URL. A route value may be an `Error` to reject, or a function
 * returning a promise when a test needs per-call behaviour.
 */
function routeHttp(routes: Record<string, unknown>): void {
  mockHttpRequest.mockImplementation((config: unknown): Promise<{data?: unknown}> => {
    const {url} = config as RecordedRequest;
    const key = Object.keys(routes).find((candidate) => url.includes(candidate));

    if (key === undefined) {
      return Promise.reject(new Error(`unexpected request: ${url}`));
    }

    const value = routes[key];

    if (value instanceof Error) {
      return Promise.reject(value);
    }
    if (typeof value === 'function') {
      return (value as () => Promise<{data?: unknown}>)();
    }

    return Promise.resolve({data: value});
  });
}

/**
 * Arranges a successful deletion, or one answered by the given value.
 */
function mockDeletion(response: unknown = {}): void {
  routeHttp({'/users/': response});
}

/**
 * The requests whose URL contains the given fragment.
 */
function requestsTo(fragment: string): RecordedRequest[] {
  return mockHttpRequest.mock.calls
    .map(([config]) => config as RecordedRequest)
    .filter((config) => config.url.includes(fragment));
}

describe('useDeleteUser', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
    mockShowToast.mockReset();
    administrationOperation = AdministrationModes.FLOW;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should show a success toast on successful deletion', async () => {
    mockDeletion();

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockDeletion(new Error('Failed to delete user'));

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
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
    mockDeletion();

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

  it('should delete through the endpoint', async () => {
    mockDeletion();

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
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
    let resolveDeletion: () => void;
    mockDeletion(
      () =>
        new Promise((resolve) => {
          resolveDeletion = () => resolve({data: {}});
        }),
    );

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveDeletion!();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete user');
    mockDeletion(apiError);

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
    mockDeletion(networkError);

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
    mockDeletion();

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
    mockDeletion();

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
    mockDeletion();

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
    mockDeletion();

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

    const deletions = requestsTo('/users/');
    expect(deletions).toHaveLength(2);
    expect(deletions[0]?.url).toBe(`https://api.test.com/users/${user1Id}`);
    expect(deletions[1]?.url).toBe(`https://api.test.com/users/${user2Id}`);
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockDeletion();

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
    mockDeletion(apiError);

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
    let attempt = 0;

    mockDeletion(() => {
      attempt += 1;
      return attempt === 1 ? Promise.reject(apiError) : Promise.resolve({data: {}});
    });

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
    expect(requestsTo('/users/')).toHaveLength(2);
  });

  it('should not affect other cached users on deletion', async () => {
    mockDeletion();

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
    mockDeletion();

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('/users/')[0]?.url).toBe(`${customServerUrl}/users/${userId}`);
  });

  it('should pass through server error messages', async () => {
    const serverError = new Error('User has active sessions and cannot be deleted');
    mockDeletion(serverError);

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(serverError);
    expect(result.current.error?.message).toBe('User has active sessions and cannot be deleted');
  });

  // The server revokes what the user holds, so native and flow modes both use the endpoint.
  it('deletes through the endpoint when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockDeletion();

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('/users/')).toHaveLength(1);
  });

  // A distributor can replace one operation without forking this package.
  it('calls the function the console supplied', async () => {
    const deleted: string[] = [];
    administrationOperation = (userId: string): Promise<void> => {
      deleted.push(userId);
      return Promise.resolve();
    };
    routeHttp({});

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(deleted).toEqual(['user-1']);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
