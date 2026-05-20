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
import AgentQueryKeys from '../../constants/agent-query-keys';
import type {Agent, UpdateAgentRequest} from '../../models/agent';
import useUpdateAgent from '../useUpdateAgent';

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

describe('useUpdateAgent', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;

  const agentId = '550e8400-e29b-41d4-a716-446655440000';
  const mockAgent: Agent = {
    id: agentId,
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Updated Service',
    description: 'Updated description',
  };

  const mockRequest: UpdateAgentRequest = {
    name: 'Updated Service',
    description: 'Updated description',
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

  it('should successfully update an agent', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result} = renderHook(() => useUpdateAgent());
    result.current.mutate({agentId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgent);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `https://api.test.com/agents/${agentId}`,
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      data: mockRequest,
    });
  });

  it('should handle API error', async () => {
    const apiError = new Error('Update failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateAgent());
    result.current.mutate({agentId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should invalidate single agent and list queries on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgent});

    const {result, queryClient} = renderHook(() => useUpdateAgent());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({agentId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENT, agentId]});
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENTS]});
  });
});
