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

const {default: useAddGroupMembers} = await import('../useAddGroupMembers');

describe('useAddGroupMembers', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should add members successfully', async () => {
    mockHttpRequest.mockResolvedValue({});
    const {result} = renderHook(() => useAddGroupMembers());

    result.current.mutate({
      groupId: 'g1',
      members: [
        {id: 'u1', type: 'user'},
        {id: 'u2', type: 'user'},
      ],
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1/members/add',
        method: 'POST',
        data: {
          members: [
            {id: 'u1', type: 'user'},
            {id: 'u2', type: 'user'},
          ],
        },
      }),
    );
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Failed to add'));
    const {result} = renderHook(() => useAddGroupMembers());

    result.current.mutate({groupId: 'g1', members: [{id: 'u1', type: 'user'}]});

    await waitFor(() => {
      expect(result.current.error?.message).toBe('Failed to add');
    });
  });
});
