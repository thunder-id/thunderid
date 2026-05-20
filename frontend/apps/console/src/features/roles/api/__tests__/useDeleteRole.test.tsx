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
  };
});

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
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve(undefined), 100);
      }),
    );

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

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

  it('should show error toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result} = renderHook(() => useDeleteRole());

    result.current.mutate('role-1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'error');
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

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-1',
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
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
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
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
});
