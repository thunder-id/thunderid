/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {Group} from '../../models/group';

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
