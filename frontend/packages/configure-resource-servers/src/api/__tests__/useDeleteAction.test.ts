// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {AdministrationModes} from '@thunderid/contexts';
import {renderHook} from '@thunderid/test-utils';
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

// How the console says this operation runs. Defaults to what a console that declares nothing gets.
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

  it('should delete a resource action through the native endpoint', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestedUrls()).toEqual([NATIVE_RESOURCE_ACTION_URL]);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: NATIVE_RESOURCE_ACTION_URL, method: 'DELETE'}),
    );
  });

  it('should delete a resource server action through the native endpoint', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteAction('rs1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestedUrls()).toEqual([NATIVE_SERVER_ACTION_URL]);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: NATIVE_SERVER_ACTION_URL, method: 'DELETE'}),
    );
  });

  it('deletes through the endpoint when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(requestedUrls()).toEqual([NATIVE_RESOURCE_ACTION_URL]);
  });

  it('should report a failed deletion', async () => {
    mockHttpRequest.mockRejectedValue(new Error('refused'));
    const {result} = renderHook(() => useDeleteAction('rs1', 'r1'));

    result.current.mutate('a1');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });
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
