// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserQueryKeys from '../../constants/user-query-keys';
import useDeleteUser from '../useDeleteUser';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
const mockShowToast = vi.fn();

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
  };
});

const FLOW_ID = '01900000-0000-7000-8000-000000000077';
const FLOW_HANDLE = 'default-user-deletion-flow';

const flowConfiguredConfig = {merged: {userDeletionFlow: {defaultHandle: FLOW_HANDLE}}};
const noFlowConfiguredConfig = {merged: {}};
const administrationFlows = {flows: [{flowType: 'ADMINISTRATION', handle: FLOW_HANDLE, id: FLOW_ID}]};

/**
 * The request shape the hook issues, as far as these tests inspect it.
 */
interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

/**
 * Answers the mocked client by URL rather than by call order.
 *
 * A deletion issues up to three requests (read the mode, resolve the flow handle, then delete), so a
 * positional mock would attach a response to the wrong call. A route value may be an `Error` to
 * reject, or a function returning a promise when a test needs per-call behaviour.
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
 * Arranges a successful flow deletion, which is what the shipped configuration produces.
 */
function mockFlowDeletion(overrides: Record<string, unknown> = {}): void {
  routeHttp({
    '/server-config/flow': flowConfiguredConfig,
    '/flows': administrationFlows,
    '/flow/execute': {flowStatus: 'COMPLETE'},
    ...overrides,
  });
}

/**
 * Arranges a successful native deletion, as a deployment with no deletion flow configured gets.
 */
function mockNativeDeletion(overrides: Record<string, unknown> = {}): void {
  routeHttp({
    '/server-config/flow': noFlowConfiguredConfig,
    '/users/': {},
    ...overrides,
  });
}

/**
 * The requests whose URL contains the given fragment, so assertions can ignore the lookups that
 * precede the deletion itself.
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
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should show a success toast on successful deletion', async () => {
    mockFlowDeletion();

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockFlowDeletion({'/flow/execute': new Error('Failed to delete user')});

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
    mockFlowDeletion();

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
    mockFlowDeletion();

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/flow/execute',
        method: 'POST',
        data: {flowId: FLOW_ID, inputs: {subject: userId}},
      }),
    );
  });

  it('should call the native endpoint when no deletion flow is configured', async () => {
    mockNativeDeletion();

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
    expect(requestsTo('/flow/execute')).toHaveLength(0);
  });

  it('should set pending state during deletion', async () => {
    mockFlowDeletion({
      '/flow/execute': () =>
        new Promise((resolve) => {
          setTimeout(() => resolve({data: {flowStatus: 'COMPLETE'}}), 100);
        }),
    });

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
      {timeout: 500},
    );

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete user');
    mockFlowDeletion({'/flow/execute': apiError});

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
    mockFlowDeletion({'/flow/execute': networkError});

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(networkError);
    expect(result.current.isPending).toBe(false);
  });

  // The flow reports a refusal in its own envelope rather than as a rejected request, so the hook has
  // to treat a non-COMPLETE terminal status as a failure.
  it('should fail when the flow reports an error status', async () => {
    mockFlowDeletion({
      '/flow/execute': {
        error: {
          code: 'FET-1084',
          message: {defaultValue: 'user has dependencies', key: 'flows.executor.errors.user_deletion_not_allowed'},
        },
        flowStatus: 'ERROR',
      },
    });

    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate('user-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toBe('user has dependencies');
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should remove user from cache on successful deletion', async () => {
    mockFlowDeletion();

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
    mockFlowDeletion();

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
    mockFlowDeletion();

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
    mockFlowDeletion();

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

    const executions = requestsTo('/flow/execute');
    expect(executions).toHaveLength(2);
    expect(executions[0]?.data).toEqual({flowId: FLOW_ID, inputs: {subject: user1Id}});
    expect(executions[1]?.data).toEqual({flowId: FLOW_ID, inputs: {subject: user2Id}});
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockFlowDeletion();

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
    mockFlowDeletion({'/flow/execute': apiError});

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

    mockFlowDeletion({
      '/flow/execute': () => {
        attempt += 1;
        return attempt === 1 ? Promise.reject(apiError) : Promise.resolve({data: {flowStatus: 'COMPLETE'}});
      },
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
    expect(requestsTo('/flow/execute')).toHaveLength(2);
  });

  it('should not affect other cached users on deletion', async () => {
    mockFlowDeletion();

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
    mockFlowDeletion();

    const userId = 'user-1';
    const {result} = renderHook(() => useDeleteUser());

    result.current.mutate(userId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestsTo('/server-config/flow')[0]?.url).toBe(`${customServerUrl}/server-config/flow`);
    expect(requestsTo('/flow/execute')[0]?.url).toBe(`${customServerUrl}/flow/execute`);
  });

  it('should pass through server error messages', async () => {
    const serverError = new Error('User has active sessions and cannot be deleted');
    mockFlowDeletion({'/flow/execute': serverError});

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
