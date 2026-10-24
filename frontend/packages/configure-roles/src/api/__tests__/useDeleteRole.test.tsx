// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {AdministrationFlowConfigKey, AdministrationFlowInput} from '@thunderid/utils';
import {describe, it, expect, beforeEach, afterEach, vi, type Mock} from 'vitest';
import RoleQueryKeys from '../../constants/role-query-keys';
import useDeleteRole from '../useDeleteRole';

// Mock the dependencies
vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
    useAdministrationOperation: () => administrationOperation,
  };
});

// How the console says this operation runs. Defaults to the flow, which is what a console that
// declares nothing gets.
let administrationOperation: unknown = AdministrationModes.FLOW;

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

describe('useDeleteRole', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn();
    mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
    mockShowToast = vi.fn();
    administrationOperation = AdministrationModes.FLOW;

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: mockGetServerUrl,
    } as unknown as ReturnType<typeof useConfig>);

    vi.mocked(useToast).mockReturnValue({
      showToast: mockShowToast,
    } as unknown as ReturnType<typeof useToast>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteRole());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully delete a role', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const roleId = 'role-1';
    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate(roleId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with role ID', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const roleId = 'role-1';
    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate(roleId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/roles/${roleId}`,
        method: 'DELETE',
      }),
    );
  });

  it('should set pending state during deletion', async () => {
    let resolveRequest: () => void;
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = () => resolve(undefined);
      }),
    );

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete role');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should handle 404 Not Found error', async () => {
    const notFoundError = new Error('Role not found');
    mockHttpRequest.mockRejectedValueOnce(notFoundError);

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('non-existent-id');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(notFoundError);
  });

  it('should remove ROLE cache entry on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const roleId = 'role-1';
    const {result, queryClient} = renderHook(() => useDeleteRole());

    queryClient.setQueryData([RoleQueryKeys.ROLE, roleId], {
      id: roleId,
      name: 'Role to Delete',
    });

    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');

    result.current.mutate(roleId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLE, roleId],
      }),
    );
  });

  it('should invalidate ROLES list cache on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const roleId = 'role-1';
    const {result, queryClient} = renderHook(() => useDeleteRole());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(roleId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLES],
      }),
    );
  });

  it('should show success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteRole());

    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should handle multiple sequential deletions', async () => {
    mockHttpRequest.mockResolvedValue(undefined);

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.mutate('role-2');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(4);
    // Calls 1 and 3 read the flow configuration; the deletions are calls 2 and 4.
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-1',
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      4,
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-2',
      }),
    );
  });

  it('should not affect other cached roles on deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteRole());

    queryClient.setQueryData([RoleQueryKeys.ROLE, 'role-1'], {id: 'role-1', name: 'Role 1'});
    queryClient.setQueryData([RoleQueryKeys.ROLE, 'role-2'], {id: 'role-2', name: 'Role 2'});

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const role2InCache = queryClient.getQueryData([RoleQueryKeys.ROLE, 'role-2']);
    expect(role2InCache).toEqual({id: 'role-2', name: 'Role 2'});
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    // The failed attempt and the retry each read the flow configuration first.
    expect(mockHttpRequest).toHaveBeenCalledTimes(3);
  });

  it('should use mutateAsync for promise-based deletion', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteRole());

    const deletePromise = result.current.mutateAsync('role-1');

    await expect(deletePromise).resolves.toBeUndefined();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  describe('when a role deletion flow is configured', () => {
    const routeRequests = (execute: () => Promise<unknown>): void => {
      (mockHttpRequest as Mock<(config: {url: string}) => Promise<unknown>>).mockImplementation((config) => {
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {merged: {[AdministrationFlowConfigKey.ROLE_DELETION]: {defaultHandle: 'delete-role'}}},
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({data: {flows: [{id: 'flow-1', handle: 'delete-role', flowType: 'ADMINISTRATION'}]}});
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }
        if (config.url.endsWith('/roles/role-1')) {
          return Promise.resolve({});
        }

        return Promise.reject(new Error(`unexpected request ${config.url}`));
      });
    };

    const requestedUrls = (): string[] => mockHttpRequest.mock.calls.map(([config]) => (config as {url: string}).url);

    it('should delete the role through the flow and never call the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteRole());

      result.current.mutate('role-1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls().filter((url) => url.endsWith('/flow/execute'))).toHaveLength(1);
      expect(requestedUrls().some((url) => url.endsWith('/roles/role-1'))).toBe(false);
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://api.test.com/flow/execute',
          data: {flowId: 'flow-1', inputs: {[AdministrationFlowInput.ROLE]: 'role-1'}},
        }),
      );
    });

    it('should report a flow refusal without falling back to the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}}));
      const {result} = renderHook(() => useDeleteRole());

      result.current.mutate('role-1');

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(requestedUrls().some((url) => url.endsWith('/roles/role-1'))).toBe(false);
    });

    it('deletes through the endpoint when the console says native', async () => {
      administrationOperation = AdministrationModes.NATIVE;
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteRole());

      result.current.mutate('role-1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls()).toEqual(['https://api.test.com/roles/role-1']);
    });

    it('calls the function the console supplied', async () => {
      const deleted: string[] = [];
      administrationOperation = (roleId: string): Promise<void> => {
        deleted.push(roleId);
        return Promise.resolve();
      };
      const {result} = renderHook(() => useDeleteRole());

      result.current.mutate('role-1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(deleted).toEqual(['role-1']);
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });
  });
});
