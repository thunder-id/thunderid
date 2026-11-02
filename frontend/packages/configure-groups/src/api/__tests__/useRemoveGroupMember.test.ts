// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {AdministrationModes} from '@thunderid/contexts';
import {renderHook} from '@thunderid/test-utils';
import {AdministrationFlowConfigKey, AdministrationFlowInput} from '@thunderid/utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
    useAdministrationOperation: () => administrationOperation,
  };
});

// How the console says this operation runs. Defaults to the flow, which is what a console that
// declares nothing gets.
let administrationOperation: unknown = AdministrationModes.FLOW;

const {default: useRemoveGroupMember} = await import('../useRemoveGroupMember');

describe('useRemoveGroupMember', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    administrationOperation = AdministrationModes.FLOW;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should remove members successfully', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useRemoveGroupMember());

    result.current.mutate({
      groupId: 'g1',
      member: {id: 'u1', type: 'user'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1/members/remove',
        method: 'POST',
        data: {members: [{id: 'u1', type: 'user'}]},
      }),
    );
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Failed to remove'));
    const {result} = renderHook(() => useRemoveGroupMember());

    result.current.mutate({groupId: 'g1', member: {id: 'u1', type: 'user'}});

    await waitFor(() => {
      expect(result.current.error?.message).toBe('Failed to remove');
    });
  });

  describe('when a membership removal flow is configured', () => {
    const routeRequests = (execute: () => Promise<unknown>): void => {
      mockHttpRequest.mockImplementation((config: {url: string}) => {
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {merged: {[AdministrationFlowConfigKey.GROUP_MEMBERSHIP_REMOVAL]: {defaultHandle: 'remove-member'}}},
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({
            data: {flows: [{id: 'flow-1', handle: 'remove-member', flowType: 'ADMINISTRATION'}]},
          });
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }

        return Promise.reject(new Error(`unexpected request ${config.url}`));
      });
    };

    it('should remove the member through one flow execution and never call the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useRemoveGroupMember());

      result.current.mutate({groupId: 'g1', member: {id: 'u1', type: 'user'}});

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const urls = mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);
      expect(urls.filter((url) => url.endsWith('/server-config/flow'))).toHaveLength(1);
      expect(urls.filter((url) => url.endsWith('/flow/execute'))).toHaveLength(1);
      expect(urls.some((url) => url.endsWith('/members/remove'))).toBe(false);
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/flow/execute',
          data: {
            flowId: 'flow-1',
            inputs: expect.objectContaining({
              [AdministrationFlowInput.GROUP]: 'g1',
              [AdministrationFlowInput.MEMBER]: 'u1',
            }) as unknown,
          },
        }),
      );
    });

    it('should report a flow refusal without falling back to the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}}));
      const {result} = renderHook(() => useRemoveGroupMember());

      result.current.mutate({groupId: 'g1', member: {id: 'u1', type: 'user'}});

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      const urls = mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);
      expect(urls.some((url) => url.endsWith('/members/remove'))).toBe(false);
    });

    it('removes through the endpoint when the console says native', async () => {
      administrationOperation = AdministrationModes.NATIVE;
      mockHttpRequest.mockResolvedValue({});
      const {result} = renderHook(() => useRemoveGroupMember());

      result.current.mutate({groupId: 'g1', member: {id: 'u1', type: 'user'}});

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      const urls = mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);
      expect(urls).toEqual(['https://localhost:8090/groups/g1/members/remove']);
    });

    it('calls the function the console supplied', async () => {
      const removed: unknown[] = [];
      administrationOperation = (variables: unknown): Promise<void> => {
        removed.push(variables);
        return Promise.resolve();
      };
      const variables = {groupId: 'g1', member: {id: 'u1', type: 'user' as const}};
      const {result} = renderHook(() => useRemoveGroupMember());

      result.current.mutate(variables);

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(removed).toEqual([variables]);
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });
  });
});
