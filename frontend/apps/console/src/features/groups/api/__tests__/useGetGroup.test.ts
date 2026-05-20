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

const {default: useGetGroup} = await import('../useGetGroup');

describe('useGetGroup', () => {
  const mockGroup: Group = {
    id: 'g1',
    name: 'Test Group',
    description: 'A test group',
    ouId: 'ou1',
    members: [{id: 'u1', type: 'user'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch a group by ID', async () => {
    mockHttpRequest.mockResolvedValue({data: mockGroup});
    const {result} = renderHook(() => useGetGroup('g1'));

    await waitFor(() => {
      expect(result.current.data).toEqual(mockGroup);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1?include=display',
        method: 'GET',
      }),
    );
  });

  it('should not fetch when groupId is empty', () => {
    const {result} = renderHook(() => useGetGroup(''));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Not found'));
    const {result} = renderHook(() => useGetGroup('g1'));

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
      expect(result.current.error?.message).toBe('Not found');
    });
  });
});
