/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import UserTypeQueryKeys from '../../constants/userTypeQueryKeys';
import useDeleteUserType from '../useDeleteUserType';

vi.mock('@thunderid/react', () => ({useThunderID: vi.fn()}));
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: vi.fn()};
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useDeleteUserType', () => {
  const mockUserTypeId = '123';

  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteUserType());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully delete a user type', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteUserType());

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/user-types/${mockUserTypeId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      }),
    );
  });

  it('should set pending state during deletion', async () => {
    let resolveRequest: (value: unknown) => void;
    const requestPromise = new Promise((resolve) => {
      resolveRequest = resolve;
    });
    mockHttpRequest.mockReturnValue(requestPromise);

    const {result} = renderHook(() => useDeleteUserType());

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    resolveRequest!(undefined);

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.isSuccess).toBe(true);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete user type');

    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteUserType());

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
    expect(result.current.isPending).toBe(false);
  });

  it('should remove user type from cache and invalidate list on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteUserType());
    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeQueriesSpy).toHaveBeenCalledWith({
      queryKey: [UserTypeQueryKeys.USER_TYPE, mockUserTypeId],
    });
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [UserTypeQueryKeys.USER_TYPES],
    });
  });

  it('should handle invalidateQueries rejection gracefully', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteUserType());
    vi.spyOn(queryClient, 'invalidateQueries').mockRejectedValueOnce(new Error('Invalidation failed'));

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteUserType());

    const promise = result.current.mutateAsync(mockUserTypeId);

    await expect(promise).resolves.toBeUndefined();

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteUserType());

    result.current.mutate(mockUserTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.data).toBeUndefined();
    });
    expect(result.current.error).toBeNull();
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
  });
});
