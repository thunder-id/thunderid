// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {AdministrationFlowConfigKey, AdministrationFlowInput} from '@thunderid/utils';
import {describe, it, expect, beforeEach, afterEach, vi, type Mock} from 'vitest';
import RoleQueryKeys from '../../constants/role-query-keys';
import useRemoveRoleAssignments from '../useRemoveRoleAssignments';

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

describe('useRemoveRoleAssignments', () => {
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
    const {result} = renderHook(() => useRemoveRoleAssignments());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(typeof result.current.mutate).toBe('function');
  });

  it('should successfully remove role assignments', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useRemoveRoleAssignments());

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

    const {result} = renderHook(() => useRemoveRoleAssignments());

    result.current.mutate({roleId: 'role-1', assignment});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

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

    const {result, queryClient} = renderHook(() => useRemoveRoleAssignments());

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

  it('should show success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useRemoveRoleAssignments());

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

    const {result} = renderHook(() => useRemoveRoleAssignments());

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

    const {result} = renderHook(() => useRemoveRoleAssignments());

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

  describe('when an assignment removal flow is configured', () => {
    const routeRequests = (execute: () => Promise<unknown>): void => {
      (mockHttpRequest as Mock<(config: {url: string}) => Promise<unknown>>).mockImplementation((config) => {
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {merged: {[AdministrationFlowConfigKey.ROLE_ASSIGNMENT_REMOVAL]: {defaultHandle: 'unassign'}}},
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({data: {flows: [{id: 'flow-1', handle: 'unassign', flowType: 'ADMINISTRATION'}]}});
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }

        return Promise.reject(new Error(`unexpected request ${config.url}`));
      });
    };

    const requestedUrls = (): string[] => mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);

    it('should remove the assignment through one flow execution and never call the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useRemoveRoleAssignments());

      result.current.mutate({roleId: 'role-1', assignment: {id: 'user-1', type: 'user'}});

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const urls = requestedUrls();
      expect(urls.filter((url) => url.endsWith('/server-config/flow'))).toHaveLength(1);
      expect(urls.filter((url) => url.endsWith('/flow/execute'))).toHaveLength(1);
      expect(urls.some((url) => url.endsWith('/assignments/remove'))).toBe(false);
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://api.test.com/flow/execute',
          data: {
            flowId: 'flow-1',
            inputs: expect.objectContaining({
              [AdministrationFlowInput.ROLE]: 'role-1',
              [AdministrationFlowInput.ASSIGNEE]: 'user-1',
            }) as unknown,
          },
        }),
      );
    });

    it('should report a flow refusal without falling back to the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}}));
      const {result} = renderHook(() => useRemoveRoleAssignments());

      result.current.mutate({roleId: 'role-1', assignment: {id: 'user-1', type: 'user'}});

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(requestedUrls().some((url) => url.endsWith('/assignments/remove'))).toBe(false);
    });
  });
});
