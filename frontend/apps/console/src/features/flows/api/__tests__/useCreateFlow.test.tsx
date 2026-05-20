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
import {FlowType, FlowNodeType} from '../../models/flows';
import type {CreateFlowRequest, FlowDefinitionResponse} from '../../models/responses';
import useCreateFlow from '../useCreateFlow';

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

describe('useCreateFlow', () => {
  const mockFlowResponse: FlowDefinitionResponse = {
    id: 'flow-new-123',
    name: 'New Login Flow',
    handle: 'new-login-flow',
    flowType: FlowType.AUTHENTICATION,
    activeVersion: 1,
    nodes: [
      {id: 'start', type: FlowNodeType.START, onSuccess: 'end'},
      {id: 'end', type: FlowNodeType.END},
    ],
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  };

  const mockCreateRequest: CreateFlowRequest = {
    name: 'New Login Flow',
    handle: 'new-login-flow',
    flowType: FlowType.AUTHENTICATION,
    nodes: [
      {id: 'start', type: FlowNodeType.START, onSuccess: 'end'},
      {id: 'end', type: FlowNodeType.END},
    ],
  };

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
    const {result} = renderHook(() => useCreateFlow());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
  });

  it('should successfully create a flow', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockFlowResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows',
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      data: mockCreateRequest,
    });
  });

  it('should set pending state during creation', async () => {
    let resolveRequest!: (value: {data: FlowDefinitionResponse}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise<{data: FlowDefinitionResponse}>((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    act(() => {
      resolveRequest({data: mockFlowResponse});
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(false);
    });
    expect(result.current.isSuccess).toBe(true);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should invalidate flows query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result, queryClient} = renderHook(() => useCreateFlow());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [FlowQueryKeys.FLOWS],
    });
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useCreateFlow());

    const promise = result.current.mutateAsync(mockCreateRequest);

    await expect(promise).resolves.toEqual(mockFlowResponse);
  });

  it('should handle onSuccess callback', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const onSuccess = vi.fn();

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest, {onSuccess});

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should handle onError callback', async () => {
    const apiError = new Error('Failed to create flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const onError = vi.fn();

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest, {onError});

    await waitFor(() => {
      expect(onError).toHaveBeenCalledWith(apiError, mockCreateRequest, undefined, expect.anything());
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.data).toBeUndefined();
    });
    expect(result.current.isIdle).toBe(true);
  });

  it('should use correct server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => customServerUrl,
    } as ReturnType<typeof useConfig>);

    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${customServerUrl}/flows`,
      }),
    );
  });

  it('should properly serialize request data as JSON', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useCreateFlow());

    result.current.mutate(mockCreateRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    const callArgs = mockHttpRequest.mock.calls[0][0] as {data: CreateFlowRequest; headers: Record<string, string>};
    expect(callArgs.data).toEqual(mockCreateRequest);
    expect(callArgs.headers['Content-Type']).toBe('application/json');
  });
});
