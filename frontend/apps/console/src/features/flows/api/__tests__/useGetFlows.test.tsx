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

import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import FlowQueryKeys from '../../constants/flow-query-keys';
import {FlowType} from '../../models/flows';
import type {FlowListResponse} from '../../models/responses';
import useGetFlows from '../useGetFlows';

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

describe('useGetFlows', () => {
  const mockFlowListResponse: FlowListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    flows: [
      {
        id: 'flow-1',
        flowType: FlowType.AUTHENTICATION,
        name: 'Basic Login Flow',
        handle: 'basic-login-flow',
        activeVersion: 1,
        createdAt: '2025-01-01T00:00:00Z',
        updatedAt: '2025-01-01T00:00:00Z',
      },
      {
        id: 'flow-2',
        flowType: FlowType.REGISTRATION,
        name: 'User Registration Flow',
        handle: 'user-registration-flow',
        activeVersion: 1,
        createdAt: '2025-01-02T00:00:00Z',
        updatedAt: '2025-01-02T00:00:00Z',
      },
    ],
    links: [
      {href: '/flows?offset=0&limit=30', rel: 'first'},
      {href: '/flows?offset=0&limit=30', rel: 'last'},
    ],
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

  it('should fetch flows with default parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowListResponse,
    });

    const {result} = renderHook(() => useGetFlows());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockFlowListResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows?limit=30&offset=0',
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should fetch flows with custom pagination parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowListResponse,
    });

    const {result} = renderHook(() => useGetFlows({limit: 10, offset: 20}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows?limit=10&offset=20',
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should fetch flows with flowType filter', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowListResponse,
    });

    const {result} = renderHook(() => useGetFlows({flowType: FlowType.AUTHENTICATION}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/flows?limit=30&offset=0&flowType=AUTHENTICATION',
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  });

  it('should show loading state initially', async () => {
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        setTimeout(() => resolve({data: mockFlowListResponse}), 100);
      }),
    );

    const {result} = renderHook(() => useGetFlows());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false);
    });
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to fetch flows');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetFlows());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should use correct query key with parameters', async () => {
    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowListResponse,
    });

    const params = {flowType: FlowType.REGISTRATION, limit: 15, offset: 5};

    const {queryClient} = renderHook(() => useGetFlows(params));

    await waitFor(() => {
      expect(mockHttpRequest).toHaveBeenCalled();
    });

    const queryState = queryClient.getQueryState([FlowQueryKeys.FLOWS, params]);
    expect(queryState).toBeDefined();
  });

  it('should use different query keys for different parameters', async () => {
    mockHttpRequest.mockResolvedValue({
      data: mockFlowListResponse,
    });

    // First render with no params
    const {result: result1} = renderHook(() => useGetFlows());

    await waitFor(() => {
      expect(result1.current.isSuccess).toBe(true);
    });

    // Second render with params
    const {result: result2} = renderHook(() => useGetFlows({flowType: FlowType.AUTHENTICATION}));

    await waitFor(() => {
      expect(result2.current.isSuccess).toBe(true);
    });

    // Both should have been called
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('should use custom server URL from config', async () => {
    const customServerUrl = 'https://custom-server.com:9090';

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => customServerUrl,
    } as ReturnType<typeof useConfig>);

    mockHttpRequest.mockResolvedValueOnce({
      data: mockFlowListResponse,
    });

    const {result} = renderHook(() => useGetFlows());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: expect.stringContaining(customServerUrl) as string,
      }),
    );
  });

  it('should return empty flows array when no flows exist', async () => {
    const emptyResponse: FlowListResponse = {
      totalResults: 0,
      startIndex: 0,
      count: 0,
      flows: [],
    };

    mockHttpRequest.mockResolvedValueOnce({
      data: emptyResponse,
    });

    const {result} = renderHook(() => useGetFlows());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.flows).toEqual([]);
    expect(result.current.data?.totalResults).toBe(0);
  });
});
