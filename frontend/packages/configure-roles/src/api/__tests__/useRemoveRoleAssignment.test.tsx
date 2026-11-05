// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AdministrationModes} from '@thunderid/contexts';
import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import RoleQueryKeys from '../../constants/role-query-keys';
import useRemoveRoleAssignment from '../useRemoveRoleAssignment';

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

describe('useRemoveRoleAssignment', () => {
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
    const {result} = renderHook(() => useRemoveRoleAssignment());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(typeof result.current.mutate).toBe('function');
  });

  it('should successfully remove role assignments', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
  });

  it('should make correct API call', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const assignment = {id: 'user-1', type: 'user' as const};

    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({roleId: 'role-1', assignment});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-1/assignments/remove',
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: {assignments: [assignment]},
      }),
    );
  });

  it('should invalidate ROLE_ASSIGNMENTS cache on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useRemoveRoleAssignment());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, 'role-1'],
      }),
    );
  });

  it('should invalidate ROLE_ASSIGNMENTS cache on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result, queryClient} = renderHook(() => useRemoveRoleAssignment());

    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith(
      expect.objectContaining({
        queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, 'role-1'],
      }),
    );
  });

  it('should show success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed'));

    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to remove assignments');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({
      roleId: 'role-1',
      assignment: {id: 'user-1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.isPending).toBe(false);
  });

  // The server revokes what the assignment granted, so native and flow modes both use the endpoint.
  it('removes through the endpoint when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockHttpRequest.mockResolvedValueOnce({});
    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate({roleId: 'role-1', assignment: {id: 'user-1', type: 'user'}});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/roles/role-1/assignments/remove', method: 'POST'}),
    );
  });

  it('calls the function the console supplied', async () => {
    const removed: unknown[] = [];
    administrationOperation = (variables: unknown): Promise<void> => {
      removed.push(variables);
      return Promise.resolve();
    };
    const variables = {roleId: 'role-1', assignment: {id: 'user-1', type: 'user' as const}};
    const {result} = renderHook(() => useRemoveRoleAssignment());

    result.current.mutate(variables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removed).toEqual([variables]);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
