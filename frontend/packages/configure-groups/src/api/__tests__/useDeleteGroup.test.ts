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

// How the console says this operation runs. A console that declares nothing gets the flow mode.
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

  it('should delete through the endpoint even when the console says flow', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteGroup());

    result.current.mutate('g1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/groups/g1', method: 'DELETE'}),
    );
  });

  it('should delete through the endpoint when the console says native', async () => {
    administrationOperation = AdministrationModes.NATIVE;
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useDeleteGroup());

    result.current.mutate('g1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/groups/g1', method: 'DELETE'}),
    );
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
