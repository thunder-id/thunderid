// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {renderHook, waitFor, act} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import FlowQueryKeys from '../../constants/flow-query-keys';
import {FlowType, FlowNodeType} from '../../models/flows';
import type {UpdateFlowRequest, FlowDefinitionResponse} from '../../models/responses';
import useUpdateFlow from '../useUpdateFlow';

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

describe('useUpdateFlow', () => {
  const mockFlowResponse: FlowDefinitionResponse = {
    id: 'flow-123',
    name: 'Updated Login Flow',
    handle: 'updated-login-flow',
    flowType: FlowType.AUTHENTICATION,
    activeVersion: 2,
    nodes: [
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT},
      {id: 'end', type: FlowNodeType.END},
    ],
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-02T00:00:00Z',
  };

  const mockUpdateRequest: UpdateFlowRequest = {
    name: 'Updated Login Flow',
    handle: 'updated-login-flow',
    flowType: FlowType.AUTHENTICATION,
    nodes: [
      {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
      {id: 'prompt', type: FlowNodeType.PROMPT},
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
    const {result} = renderHook(() => useUpdateFlow());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
  });

  it('should successfully update a flow', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockFlowResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows/flow-123',
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      data: mockUpdateRequest,
    });
  });

  it('should set pending state during update', async () => {
    let resolveRequest!: (value: {data: FlowDefinitionResponse}) => void;

    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

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
    const apiError = new Error('Failed to update flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should invalidate flows list query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result, queryClient} = renderHook(() => useUpdateFlow());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [FlowQueryKeys.FLOWS],
    });
  });

  it('should invalidate specific flow query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result, queryClient} = renderHook(() => useUpdateFlow());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [FlowQueryKeys.FLOW, 'flow-123'],
    });
  });

  it('should support mutateAsync for promise-based workflows', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useUpdateFlow());

    const promise = result.current.mutateAsync({flowId: 'flow-123', flowData: mockUpdateRequest});

    await expect(promise).resolves.toEqual(mockFlowResponse);
  });

  it('should handle onSuccess callback', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const onSuccess = vi.fn();

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest}, {onSuccess});

    await waitFor(() => {
      expect(onSuccess).toHaveBeenCalled();
    });
  });

  it('should handle onError callback', async () => {
    const apiError = new Error('Failed to update flow');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const onError = vi.fn();

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest}, {onError});

    await waitFor(() => {
      expect(onError).toHaveBeenCalled();
    });
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse});

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

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

    const {result} = renderHook(() => useUpdateFlow());

    result.current.mutate({flowId: 'flow-123', flowData: mockUpdateRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `${customServerUrl}/flows/flow-123`,
      }),
    );
  });
});
