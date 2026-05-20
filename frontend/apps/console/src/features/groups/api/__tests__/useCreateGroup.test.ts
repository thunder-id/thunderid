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
import type {CreateGroupRequest} from '../../models/requests';

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

const {default: useCreateGroup} = await import('../useCreateGroup');

describe('useCreateGroup', () => {
  const mockGroup: Group = {
    id: 'g1',
    name: 'New Group',
    ouId: 'ou1',
  };

  const mockRequest: CreateGroupRequest = {
    name: 'New Group',
    ouId: 'ou1',
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useCreateGroup());

    expect(result.current.isIdle).toBe(true);
    expect(result.current.data).toBeUndefined();
  });

  it('should create a group successfully', async () => {
    mockHttpRequest.mockResolvedValue({data: mockGroup});
    const {result} = renderHook(() => useCreateGroup());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.data).toEqual(mockGroup);
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://localhost:8090/groups',
        method: 'POST',
        data: mockRequest,
      }),
    );
  });

  it('should handle error', async () => {
    mockHttpRequest.mockRejectedValue(new Error('Validation failed'));
    const {result} = renderHook(() => useCreateGroup());

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.error).toBeTruthy();
      expect(result.current.error?.message).toBe('Validation failed');
    });
  });

  it('should set pending state during mutation', async () => {
    let resolveRequest: (value: unknown) => void;
    const requestPromise = new Promise((resolve) => {
      resolveRequest = resolve;
    });
    mockHttpRequest.mockReturnValue(requestPromise);

    const {result} = renderHook(() => useCreateGroup());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest!({data: mockGroup});

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
  });
});
