// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
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
    useAdministrationOperation: () => administrationOperation,
  };
});

// How the console says this operation runs. A console that declares nothing gets the flow mode.
let administrationOperation: unknown = AdministrationModes.FLOW;

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
    mockHttpRequest.mockResolvedValueOnce({
      data: mockUpdatedRole,
    });

    const {result} = renderHook(() => useUpdateRole());

    result.current.mutate({roleId: 'role-1', data: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    // eslint-disable-next-line @typescript-eslint/no-unsafe-assignment
    const callArgs = mockHttpRequest.mock.calls[0][0];
    // eslint-disable-next-line @typescript-eslint/no-unsafe-member-access
    expect(callArgs.data).toEqual(mockUpdateRequest);
  });

  it('should clear error state on successful retry', async () => {
    const apiError = new Error('Temporary error');
    mockHttpRequest.mockRejectedValueOnce(apiError).mockResolvedValueOnce({data: mockUpdatedRole});

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

  describe('when the update removes a permission', () => {
    const storedRole: Role = {
      id: 'role-1',
      name: 'Original Role',
      description: 'Original description',
      ouId: 'ou-1',
      permissions: [{resourceServerId: 'rs-1', permissions: ['read', 'write']}],
    };
    const narrowed: UpdateRoleRequest = {
      name: 'Renamed Role',
      description: storedRole.description,
      ouId: 'ou-1',
      permissions: [{resourceServerId: 'rs-1', permissions: ['read']}],
    };

    // The server revokes what a dropped permission granted, so flow and native modes both use the endpoint.
    it.each([AdministrationModes.FLOW, AdministrationModes.NATIVE])(
      'should write the whole role through the endpoint in %s mode',
      async (operation) => {
        administrationOperation = operation;
        mockHttpRequest.mockResolvedValueOnce({data: {...storedRole, ...narrowed}});
        const {result} = renderHook(() => useUpdateRole());

        result.current.mutate({roleId: 'role-1', data: narrowed});

        await waitFor(() => {
          expect(result.current.isSuccess).toBe(true);
        });

        expect(mockHttpRequest).toHaveBeenCalledTimes(1);
        expect(mockHttpRequest).toHaveBeenCalledWith(
          expect.objectContaining({url: 'https://api.test.com/roles/role-1', method: 'PUT', data: narrowed}),
        );
      },
    );

    it('calls the function the console supplied', async () => {
      const updated: unknown[] = [];
      administrationOperation = (variables: unknown): Promise<Role> => {
        updated.push(variables);
        return Promise.resolve(storedRole);
      };
      const variables = {roleId: 'role-1', data: narrowed};
      const {result} = renderHook(() => useUpdateRole());

      result.current.mutate(variables);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(updated).toEqual([variables]);
      expect(result.current.data).toEqual(storedRole);
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });
  });
});
