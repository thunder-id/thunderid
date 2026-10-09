// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {AdministrationFlowConfigKey, AdministrationFlowInput} from '@thunderid/utils';
import {describe, it, expect, beforeEach, afterEach, vi, type Mock} from 'vitest';
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

  // Nothing is removed, so the update takes the native path these tests cover.
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

    // calls[0] reads the current role.
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

  describe('when the edit removes a permission', () => {
    const storedRole: Role = {
      id: 'role-1',
      name: 'Original Role',
      description: 'Original description',
      ouId: 'ou-1',
      permissions: [{resourceServerId: 'rs-1', permissions: ['read', 'write']}],
    };
    const reducedPermissions: Role['permissions'] = [{resourceServerId: 'rs-1', permissions: ['read']}];

    interface RequestConfig {
      url: string;
      method: string;
      data?: UpdateRoleRequest;
    }

    interface RouteOptions {
      flowConfigured?: boolean;
      put?: (body: UpdateRoleRequest) => Promise<unknown>;
      execute?: () => Promise<unknown>;
    }

    const routeRequests = ({
      flowConfigured = true,
      put = (body) => Promise.resolve({data: {...storedRole, ...body}}),
      execute = () => Promise.resolve({data: {flowStatus: 'COMPLETE'}}),
    }: RouteOptions = {}): void => {
      (mockHttpRequest as Mock<(config: RequestConfig) => Promise<unknown>>).mockImplementation((config) => {
        if (config.url.endsWith('/roles/role-1') && config.method === 'GET') {
          return Promise.resolve({data: storedRole});
        }
        if (config.url.endsWith('/roles/role-1') && config.method === 'PUT') {
          return put(config.data!);
        }
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {
              merged: flowConfigured
                ? {[AdministrationFlowConfigKey.ROLE_PERMISSION_REMOVAL]: {defaultHandle: 'drop-permissions'}}
                : {},
            },
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({
            data: {flows: [{id: 'flow-1', handle: 'drop-permissions', flowType: 'ADMINISTRATION'}]},
          });
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }

        return Promise.reject(new Error(`unexpected request ${config.method} ${config.url}`));
      });
    };

    const requests = (): RequestConfig[] => mockHttpRequest.mock.calls.map(([config]: [RequestConfig]) => config);
    const puts = () => requests().filter((config) => config.method === 'PUT');
    const executions = () => requests().filter((config) => config.url.endsWith('/flow/execute'));

    it('should run only the flow when nothing but the permissions changed', async () => {
      routeRequests();
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate({
        roleId: 'role-1',
        data: {
          name: storedRole.name,
          description: storedRole.description,
          ouId: 'ou-1',
          permissions: reducedPermissions,
        },
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(puts()).toHaveLength(0);
      expect(executions()).toHaveLength(1);
      expect(executions()[0].data).toEqual({
        flowId: 'flow-1',
        inputs: {
          [AdministrationFlowInput.PERMISSIONS]: JSON.stringify(reducedPermissions),
          [AdministrationFlowInput.ROLE]: 'role-1',
        },
      });
      expect(result.current.data).toEqual({...storedRole, permissions: reducedPermissions});
    });

    it('should report a flow failure without writing the role when only the permissions changed', async () => {
      routeRequests({execute: () => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}})});
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate({
        roleId: 'role-1',
        data: {
          name: storedRole.name,
          description: storedRole.description,
          ouId: 'ou-1',
          permissions: reducedPermissions,
        },
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(puts()).toHaveLength(0);
    });

    it('should write the other fields with the existing permissions before running the flow', async () => {
      routeRequests();
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate({
        roleId: 'role-1',
        data: {name: 'Renamed Role', description: 'New description', ouId: 'ou-1', permissions: reducedPermissions},
      });

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(puts()).toHaveLength(1);
      expect(puts()[0].data).toEqual({
        name: 'Renamed Role',
        description: 'New description',
        ouId: 'ou-1',
        permissions: storedRole.permissions,
      });
      expect(executions()).toHaveLength(1);
      const putIndex = requests().findIndex((config) => config.method === 'PUT');
      const executeIndex = requests().findIndex((config) => config.url.endsWith('/flow/execute'));
      expect(putIndex).toBeLessThan(executeIndex);
      expect(result.current.data).toEqual({
        ...storedRole,
        name: 'Renamed Role',
        description: 'New description',
        permissions: reducedPermissions,
      });
    });

    it('should not run the flow when writing the other fields fails', async () => {
      const apiError = new Error('Role name already exists');
      routeRequests({put: () => Promise.reject(apiError)});
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate({
        roleId: 'role-1',
        data: {name: 'Taken Name', description: storedRole.description, ouId: 'ou-1', permissions: reducedPermissions},
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(result.current.error).toEqual(apiError);
      expect(executions()).toHaveLength(0);
    });

    it('should report a flow failure after the other fields were written with the existing permissions', async () => {
      routeRequests({execute: () => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}})});
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate({
        roleId: 'role-1',
        data: {
          name: 'Renamed Role',
          description: storedRole.description,
          ouId: 'ou-1',
          permissions: reducedPermissions,
        },
      });

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(puts()).toHaveLength(1);
      expect(puts()[0].data?.permissions).toEqual(storedRole.permissions);
    });

    it('should update the role natively in a single write when no flow is configured', async () => {
      routeRequests({flowConfigured: false});
      const {result} = renderHook(() => useUpdateRole());
      const data: UpdateRoleRequest = {
        name: 'Renamed Role',
        description: storedRole.description,
        ouId: 'ou-1',
        permissions: reducedPermissions,
      };

      result.current.mutate({roleId: 'role-1', data});

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(puts()).toHaveLength(1);
      expect(puts()[0].data).toEqual(data);
      expect(executions()).toHaveLength(0);
    });
  });
});
