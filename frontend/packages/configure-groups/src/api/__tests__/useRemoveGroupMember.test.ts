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

const {default: useRemoveGroupMember} = await import('../useRemoveGroupMember');

describe('useRemoveGroupMember', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    administrationOperation = AdministrationModes.FLOW;
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should remove the member through the endpoint', async () => {
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

  describe('when the console declares how the removal runs', () => {
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
