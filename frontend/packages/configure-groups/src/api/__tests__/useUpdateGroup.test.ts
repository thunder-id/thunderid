// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {Group} from '../../models/group';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  GroupQueryKeys: {GROUPS: 'groups'},
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
  };
});

const {default: useUpdateGroup} = await import('../useUpdateGroup');

describe('useUpdateGroup', () => {
  const mockUpdatedGroup: Group = {
    id: 'g1',
    name: 'Updated Group',
    description: 'Updated desc',
    ouId: 'ou1',
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should update a group successfully', async () => {
    mockHttpRequest.mockResolvedValue({data: mockUpdatedGroup});
    const {result} = renderHook(() => useUpdateGroup());

    result.current.mutate({
      groupId: 'g1',
      data: {name: 'Updated Group', description: 'Updated desc', ouId: 'ou1'},
    });

    await waitFor(() => {
      expect(result.current.data).toEqual(mockUpdatedGroup);
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1',
        method: 'PUT',
      }),
    );
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Update failed'));
    const {result} = renderHook(() => useUpdateGroup());

    result.current.mutate({
      groupId: 'g1',
      data: {name: 'Updated', ouId: 'ou1'},
    });

    await waitFor(() => {
      expect(result.current.error?.message).toBe('Update failed');
    });
  });
});
