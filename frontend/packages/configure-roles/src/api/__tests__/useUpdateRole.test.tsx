// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import RoleQueryKeys from '../../constants/role-query-keys';
import type {UpdateRoleRequest} from '../../models/requests';
import type {Role} from '../../models/role';
import useUpdateRole, {ROLE_MUTATION_KEY} from '../useUpdateRole';

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

describe('useUpdateRole', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockGetServerUrl: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;

  const mockUpdatedRole: Role = {
    id: 'role-1',
    name: 'Updated Role',
    description: 'Updated description',
    ouId: 'ou-1',
    permissions: [
      {
        resourceServerId: 'rs-1',
        permissions: ['read', 'write', 'delete'],
      },
    ],
  };

  const mockUpdateRequest: UpdateRoleRequest = {
    name: 'Updated Role',
    description: 'Updated description',
    ouId: 'ou-1',
    permissions: [
      {
        resourceServerId: 'rs-1',
        permissions: ['read', 'write', 'delete'],
      },
    ],
  };

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

  /**
   * Answers the read the hook performs before updating, reporting the permissions the update is about
   * to send. Nothing is removed, so no revocation flow runs and the update proceeds natively, which is
   * the path these tests cover.
   */
  const queueUnchangedPermissionsRead = (permissions: Role['permissions'] = mockUpdatedRole.permissions): void => {
    mockHttpRequest.mockResolvedValueOnce({data: {...mockUpdatedRole, permissions}});
  };

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useUpdateRole());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully update a role', async () => {
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockUpdatedRole);
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call with role ID in URL', async () => {
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-1',
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: mockUpdateRequest,
      }),
    );
  });

  it('should set pending state during update', async () => {
    let resolveRequest: ((value: {data: Role}) => void) | undefined;
    mockHttpRequest.mockReturnValue(
      new Promise<{data: Role}>((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest?.({data: mockUpdatedRole});

    await waitFor(
      () => {
        expect(result.current.isSuccess).toBe(true);
      },
      {timeout: 200},
    );

    expect(result.current.isPending).toBe(false);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to update role');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should invalidate ROLE cache for specific roleId on success', async () => {
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result, queryClient} = renderHook(() => useUpdateRole());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLE, 'role-1'],
      }),
    );
  });

  it('should invalidate ROLES list cache on success', async () => {
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result, queryClient} = renderHook(() => useUpdateRole());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

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
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should send JSON-stringified data in request body', async () => {
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // The update is the second request: the first reads the role to decide whether the edit removes a
    // permission.
    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[1][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.data).toEqual(mockUpdateRequest);
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockRejectedValueOnce(apiError);
    queueUnchangedPermissionsRead();
    mockHttpRequest.mockResolvedValueOnce({data: mockUpdatedRole});

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
  });

  it('should merge PUT response into cache while preserving display-only fields like ouHandle', async () => {
    const responseRole: Role = {
      id: 'role-1',
      name: 'Updated Role',
      description: 'Updated description',
      ouId: 'ou-1',
      permissions: [{resourceServerId: 'rs-1', permissions: ['read']}],
    };
    queueUnchangedPermissionsRead(responseRole.permissions);
    mockHttpRequest.mockResolvedValueOnce({data: responseRole});

    const {result, queryClient} = renderHook(() => useUpdateRole());

    const cachedRoleWithDisplay: Role = {
      id: 'role-1',
      name: 'Original Role',
      description: 'Original description',
      ouId: 'ou-1',
      ouHandle: 'my-org',
      permissions: [{resourceServerId: 'rs-1', permissions: ['read', 'write']}],
    };
    queryClient.setQueryData([RoleQueryKeys.ROLE, 'role-1'], cachedRoleWithDisplay);

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const cached = queryClient.getQueryData<Role>([RoleQueryKeys.ROLE, 'role-1']);
    expect(cached?.name).toBe('Updated Role');
    expect(cached?.description).toBe('Updated description');
    expect(cached?.permissions).toEqual([{resourceServerId: 'rs-1', permissions: ['read']}]);
    expect(cached?.ouHandle).toBe('my-org');
  });

  it('should register the mutation under ROLE_MUTATION_KEY', () => {
    const {result} = renderHook(() => useUpdateRole());

    expect(result.current).toBeDefined();
    expect(ROLE_MUTATION_KEY).toEqual(['update-role']);
  });
});
