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

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentQueryKeys from '../../constants/agent-query-keys';
import type {Agent, CreateAgentRequest} from '../../models/agent';
import useCreateAgent from '../useCreateAgent';

vi.mock('@thunderid/react', () => ({
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn().mockReturnValue({showToast: vi.fn()}),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useCreateAgent', () => {
  const mockAgent: Agent = {
    id: '550e8400-e29b-41d4-a716-446655440000',
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Billing Service',
    description: 'Service-to-service billing agent',
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'agent-billing',
          clientSecret: 'super-secret',
          grantTypes: ['client_credentials'],
          tokenEndpointAuthMethod: 'client_secret_basic',
          responseTypes: [],
          token: {
            accessToken: {validityPeriod: 3600, userAttributes: []},
            idToken: {validityPeriod: 3600, userAttributes: []},
          },
        },
      },
    ],
  };

  const mockRequest: CreateAgentRequest = {
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Billing Service',
    description: 'Service-to-service billing agent',
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          grantTypes: ['client_credentials'],
          tokenEndpointAuthMethod: 'client_secret_basic',
          responseTypes: [],
          token: {
            accessToken: {validityPeriod: 3600, userAttributes: []},
            idToken: {validityPeriod: 3600, userAttributes: []},
          },
        },
      },
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
    const {result} = renderHook(() => useCreateAgent());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
    expect(result.current.isSuccess).toBe(false);
    expect(result.current.isError).toBe(false);
    expect(typeof result.current.mutate).toBe('function');
    expect(typeof result.current.mutateAsync).toBe('function');
  });

  it('should successfully create an agent', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useCreateAgent());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgent);
    expect(result.current.error).toBeNull();
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://localhost:8090/agents',
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      data: mockRequest,
    });
  });

  it('should set pending state during creation', async () => {
    let resolveRequest!: (value: {data: Agent}) => void;
    mockHttpRequest.mockReturnValue(
      new Promise((resolve) => {
        resolveRequest = resolve;
      }),
    );

    const {result} = renderHook(() => useCreateAgent());
    act(() => {
      result.current.mutate(mockRequest);
    });

    await waitFor(() => {
      expect(result.current.isPending).toBe(true);
    });

    act(() => {
      resolveRequest({data: mockAgent});
    });

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });
  });

  it('should handle API error', async () => {
    const apiError = new Error('Failed to create agent');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateAgent());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(result.current.data).toBeUndefined();
  });

  it('should invalidate agents list query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result, queryClient} = renderHook(() => useCreateAgent());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENTS]});
  });

  it('should support mutateAsync', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useCreateAgent());
    await expect(result.current.mutateAsync(mockRequest)).resolves.toEqual(mockAgent);
  });

  it('should reset mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useCreateAgent());
    result.current.mutate(mockRequest);

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
});
