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

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {AgentListResponse} from '../../models/agent';
import useGetAgents from '../useGetAgents';

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

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useGetAgents', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  const mockResponse: AgentListResponse = {
    totalResults: 2,
    startIndex: 1,
    count: 2,
    agents: [
      {
        id: '550e8400-e29b-41d4-a716-446655440000',
        ouId: '111e8400-e29b-41d4-a716-446655440000',
        type: 'default',
        name: 'Billing Service',
      },
      {
        id: '660e8400-e29b-41d4-a716-446655440001',
        ouId: '111e8400-e29b-41d4-a716-446655440000',
        type: 'default',
        name: 'Reports Service',
      },
    ],
  };

  beforeEach(() => {
    mockHttpRequest = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with loading state', () => {
    mockHttpRequest.mockReturnValue(new Promise(() => null));

    const {result} = renderHook(() => useGetAgents());

    expect(result.current.isLoading).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should successfully fetch agents list with default pagination', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockResponse});

    const {result} = renderHook(() => useGetAgents());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockResponse);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/agents?limit=30&offset=0&include=display',
        method: 'GET',
      }),
    );
  });

  it('should pass custom pagination params', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockResponse});

    const {result} = renderHook(() => useGetAgents({limit: 50, offset: 100}));

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/agents?limit=50&offset=100&include=display',
      }),
    );
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to load agents');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useGetAgents());

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should return empty list when no agents exist', async () => {
    const emptyResponse: AgentListResponse = {totalResults: 0, startIndex: 1, count: 0, agents: []};
    mockHttpRequest.mockResolvedValueOnce({data: emptyResponse});

    const {result} = renderHook(() => useGetAgents());

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(emptyResponse);
    expect(result.current.data?.agents).toHaveLength(0);
  });
});
