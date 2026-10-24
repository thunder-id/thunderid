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

const {default: useDeleteGroup} = await import('../useDeleteGroup');

describe('useDeleteGroup', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    administrationOperation = AdministrationModes.FLOW;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should delete a group successfully', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteGroup());

    result.current.mutate('g1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1',
        method: 'DELETE',
      }),
    );
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Delete failed'));
    const {result} = renderHook(() => useDeleteGroup());

    result.current.mutate('g1');

    await waitFor(() => {
      expect(result.current.error?.message).toBe('Delete failed');
    });
  });

  describe('when a group deletion flow is configured', () => {
    const routeRequests = (execute: () => Promise<unknown>): void => {
      mockHttpRequest.mockImplementation((config: {url: string}) => {
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {merged: {[AdministrationFlowConfigKey.GROUP_DELETION]: {defaultHandle: 'delete-group'}}},
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({
            data: {flows: [{id: 'flow-1', handle: 'delete-group', flowType: 'ADMINISTRATION'}]},
          });
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }
        if (config.url.endsWith('/groups/g1')) {
          return Promise.resolve({});
        }

        return Promise.reject(new Error(`unexpected request ${config.url}`));
      });
    };

    const requestedUrls = (): string[] => mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);

    it('should delete the group through the flow and never call the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteGroup());

      result.current.mutate('g1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls().some((url) => url.endsWith('/groups/g1'))).toBe(false);
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/flow/execute',
          data: {flowId: 'flow-1', inputs: {[AdministrationFlowInput.GROUP]: 'g1'}},
        }),
      );
    });

    it('should report a flow refusal without falling back to the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}}));
      const {result} = renderHook(() => useDeleteGroup());

      result.current.mutate('g1');

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(requestedUrls().some((url) => url.endsWith('/groups/g1'))).toBe(false);
    });

    it('deletes through the endpoint when the console says native', async () => {
      administrationOperation = AdministrationModes.NATIVE;
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteGroup());

      result.current.mutate('g1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls()).toEqual(['https://localhost:8090/groups/g1']);
    });

    it('calls the function the console supplied', async () => {
      const deleted: string[] = [];
      administrationOperation = (groupId: string): Promise<void> => {
        deleted.push(groupId);
        return Promise.resolve();
      };
      const {result} = renderHook(() => useDeleteGroup());

      result.current.mutate('g1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(deleted).toEqual(['g1']);
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });
  });
});
