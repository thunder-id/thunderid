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

import {useThunderID} from '@thunderid/react';
import {useConfig} from '@thunderid/contexts';
import {renderHook, waitFor, act} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import FlowQueryKeys from '../../constants/flow-query-keys';
import useDeleteFlow from '../useDeleteFlow';

vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
  };
});

describe('useDeleteFlow', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://localhost:8090',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteFlow());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
  });

  it('should successfully delete a flow', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows/flow-123',
      method: 'DELETE',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('should set pending state during deletion', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve({}), 100);
      }),
    );

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });

    expect(result.current.isSuccess).toBe(true);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to delete flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should remove specific flow from cache on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const {result, queryClient} = renderHook(() => useDeleteFlow());
    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeQueriesSpy).toHaveBeenCalledWith({
      queryKey: [FlowQueryKeys.FLOW, 'flow-123'],
    });
  });

  it('should invalidate flows list query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const {result, queryClient} = renderHook(() => useDeleteFlow());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [FlowQueryKeys.FLOWS],
    });
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const {result} = renderHook(() => useDeleteFlow());

    const promise = result.current.mutateAsync('flow-123');

    await expect(promise).resolves.toBeUndefined();
  });

  it('should handle onSuccess callback', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const onSuccess = vi.fn();

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123', {onSuccess});

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should handle onError callback', async () => {
    const apiError = new Error('Failed to delete flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const onError = vi.fn();

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123', {onError});

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith(apiError, 'flow-123', undefined, expect.anything());
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({});

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-123');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.isIdle).toBe(true);
    });
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => customServerUrl,
    } as ReturnType<typeof useConfig>);

    mockHttpRequest.mockResolvedValueOnce({});

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-456');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `${customServerUrl}/flows/flow-456`,
      method: 'DELETE',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('should handle multiple sequential deletions', async () => {
    mockHttpRequest.mockResolvedValue({});

    const {result} = renderHook(() => useDeleteFlow());

    result.current.mutate('flow-1');

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    result.current.mutate('flow-2');

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    });
  });
});
