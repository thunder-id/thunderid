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

const {default: useDeleteAction} = await import('../useDeleteAction');

const NATIVE_RESOURCE_ACTION_URL = 'https://localhost:8090/resource-servers/rs1/resources/r1/actions/a1';
const NATIVE_SERVER_ACTION_URL = 'https://localhost:8090/resource-servers/rs1/actions/a1';

describe('useDeleteAction', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    administrationOperation = AdministrationModes.FLOW;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  const requestedUrls = (): string[] => mockHttpRequest.mock.calls.map(([config]: [{url: string}]) => config.url);

  it('should delete a resource action through the native endpoint when no flow is configured', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: NATIVE_RESOURCE_ACTION_URL, method: 'DELETE'}),
    );
  });

  it('should delete a resource server action through the native endpoint when no flow is configured', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteAction('rs1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: NATIVE_SERVER_ACTION_URL, method: 'DELETE'}),
    );
  });

  describe('when an action deletion flow is configured', () => {
    const routeRequests = (execute: () => Promise<unknown>): void => {
      mockHttpRequest.mockImplementation((config: {url: string}) => {
        if (config.url.endsWith('/server-config/flow')) {
          return Promise.resolve({
            data: {merged: {[AdministrationFlowConfigKey.ACTION_DELETION]: {defaultHandle: 'delete-action'}}},
          });
        }
        if (config.url.includes('/flows?')) {
          return Promise.resolve({
            data: {flows: [{id: 'flow-1', handle: 'delete-action', flowType: 'ADMINISTRATION'}]},
          });
        }
        if (config.url.endsWith('/flow/execute')) {
          return execute();
        }
        if (config.url === NATIVE_RESOURCE_ACTION_URL) {
          return Promise.resolve({});
        }

        return Promise.reject(new Error(`unexpected request ${config.url}`));
      });
    };

    it('should delete the action through the flow and never call the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

      result.current.mutate('a1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls()).not.toContain(NATIVE_RESOURCE_ACTION_URL);
      expect(mockHttpRequest).toHaveBeenCalledWith(
        expect.objectContaining({
          url: 'https://localhost:8090/flow/execute',
          data: {
            flowId: 'flow-1',
            inputs: {
              [AdministrationFlowInput.ACTION]: 'a1',
              [AdministrationFlowInput.RESOURCE_SERVER]: 'rs1',
              [AdministrationFlowInput.RESOURCE]: 'r1',
            },
          },
        }),
      );
    });

    it('should report a flow refusal without falling back to the native endpoint', async () => {
      routeRequests(() => Promise.resolve({data: {flowStatus: 'ERROR', error: {}}}));
      const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

      result.current.mutate('a1');

      await waitFor(() => {
        expect(result.current.isError).toBe(true);
      });

      expect(requestedUrls()).not.toContain(NATIVE_RESOURCE_ACTION_URL);
    });

    it('deletes through the endpoint when the console says native', async () => {
      administrationOperation = AdministrationModes.NATIVE;
      routeRequests(() => Promise.resolve({data: {flowStatus: 'COMPLETE'}}));
      const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

      result.current.mutate('a1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(requestedUrls()).toEqual([NATIVE_RESOURCE_ACTION_URL]);
    });

    it('calls the function the console supplied', async () => {
      const deleted: unknown[][] = [];
      administrationOperation = (...args: unknown[]): Promise<void> => {
        deleted.push(args);
        return Promise.resolve();
      };
      const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

      result.current.mutate('a1');

      await waitFor(() => {
        expect(result.current.isSuccess).toBe(true);
      });

      expect(deleted).toEqual([['rs1', 'a1', 'r1']]);
      expect(mockHttpRequest).not.toHaveBeenCalled();
    });
  });
});
