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
import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import FlowQueryKeys from '../../constants/flow-query-keys';
import {FlowType, FlowNodeType} from '../../models/flows';
import type {FlowDefinitionResponse} from '../../models/responses';
import useGetFlowById from '../useGetFlowById';

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

describe('useGetFlowById', () => {
  const mockFlowResponse: FlowDefinitionResponse = {
    id: 'flow-123',
    name: 'Basic Login Flow',
    handle: 'basic-login-flow',
    flowType: FlowType.AUTHENTICATION,
    activeVersion: 1,
    nodes: [
      {
        id: 'node-start',
        type: FlowNodeType.START,
        onSuccess: 'node-prompt',
      },
      {
        id: 'node-prompt',
        type: FlowNodeType.PROMPT,
        meta: {
          components: [],
        },
        prompts: [],
      },
      {
        id: 'node-end',
        type: FlowNodeType.END,
      },
    ],
    createdAt: '2025-01-01T00:00:00Z',
    updatedAt: '2025-01-01T00:00:00Z',
  };

  let mockHttpRequest: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {
        request: mockHttpRequest,
      },
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://localhost:8090',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should fetch flow by ID successfully', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowResponse,
    });

    const {result} = renderHook(() => useGetFlowById('flow-123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockFlowResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows/flow-123',
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should not fetch when flowId is undefined', () => {
    const {result} = renderHook(() => useGetFlowById(undefined));

    // Should not be loading or fetching when disabled
    expect(result.current.isFetching).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should not fetch when enabled is false', () => {
    const {result} = renderHook(() => useGetFlowById('flow-123', false));

    expect(result.current.isFetching).toBe(false);
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('should fetch when enabled changes to true', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowResponse,
    });

    const {result, rerender} = renderHook(({flowId, enabled}) => useGetFlowById(flowId, enabled), {
      initialProps: {flowId: 'flow-123', enabled: false},
    });

    expect(mockHttpRequest).not.toHaveBeenCalled();

    rerender({flowId: 'flow-123', enabled: true});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalled();
  });

  it('should show loading state while fetching', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve({data: mockFlowResponse}), 100);
      }),
    );

    const {result} = renderHook(() => useGetFlowById('flow-123'));

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });

    expect(result.current.data).toEqual(mockFlowResponse);
  });

  it('should handle API error', async () => {
    const apiError = new Error('Flow not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetFlowById('non-existent-flow'));

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should use correct query key', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowResponse,
    });

    const {queryClient} = renderHook(() => useGetFlowById('flow-123'));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });

    const queryState = queryClient.getQueryState([FlowQueryKeys.FLOW, 'flow-123']);
    expect(queryState).toBeDefined();
  });

  it('should refetch when flowId changes', async () => {
    const secondFlowResponse: FlowDefinitionResponse = {
      ...mockFlowResponse,
      id: 'flow-456',
      name: 'Second Flow',
    };

    mockHttpRequest.mockResolvedValueOnce({data: mockFlowResponse}).mockResolvedValueOnce({data: secondFlowResponse});

    const {result, rerender} = renderHook(({flowId}) => useGetFlowById(flowId), {
      initialProps: {flowId: 'flow-123'},
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.id).toBe('flow-123');

    rerender({flowId: 'flow-456'});

    await waitFor(() => {
      expect(result.current.data?.id).toBe('flow-456');
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should use custom server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => customServerUrl,
    } as ReturnType<typeof useConfig>);

    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowResponse,
    });

    const {result} = renderHook(() => useGetFlowById('flow-123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `${customServerUrl}/flows/flow-123`,
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should return flow with all node types', async () => {
    const flowWithAllNodeTypes: FlowDefinitionResponse = {
      ...mockFlowResponse,
      nodes: [
        {id: 'start', type: FlowNodeType.START, onSuccess: 'prompt'},
        {id: 'prompt', type: FlowNodeType.PROMPT, prompts: [{action: {ref: 'btn', nextNode: 'exec'}}]},
        {id: 'exec', type: FlowNodeType.TASK_EXECUTION, executor: {name: 'TestExecutor'}, onSuccess: 'end'},
        {id: 'end', type: FlowNodeType.END},
      ],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: flowWithAllNodeTypes,
    });

    const {result} = renderHook(() => useGetFlowById('flow-123'));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.nodes).toHaveLength(4);
    expect(result.current.data?.nodes.map((n) => n.type)).toEqual([
      FlowNodeType.START,
      FlowNodeType.PROMPT,
      FlowNodeType.TASK_EXECUTION,
      FlowNodeType.END,
    ]);
  });
});
