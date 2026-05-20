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
import AgentTypeQueryKeys from '../../constants/agentTypeQueryKeys';
import type {ApiAgentType} from '../../models/agent-type';
import type {UpdateAgentTypeRequest} from '../../models/requests';
import useUpdateAgentType from '../useUpdateAgentType';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
    useToast: vi.fn().mockReturnValue({showToast: vi.fn()}),
  };
});

describe('useUpdateAgentType', () => {
  const agentTypeId = 'aaa-bbb-ccc';
  const ouId = '111e8400-e29b-41d4-a716-446655440000';

  const mockAgentType: ApiAgentType = {
    id: agentTypeId,
    name: 'default',
    ouId,
    schema: {model: {type: 'string', required: false}},
  };

  const mockRequest: UpdateAgentTypeRequest = {
    name: 'default',
    ouId,
    schema: {model: {type: 'string', required: false}},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should successfully update an agent type', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result} = renderHook(() => useUpdateAgentType());
    result.current.mutate({agentTypeId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgentType);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `https://api.test.com/agent-types/${agentTypeId}`,
      method: 'PUT',
      headers: {'Content-Type': 'application/json'},
      data: mockRequest,
    });
  });

  it('should handle API error', async () => {
    const apiError = new Error('Update failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useUpdateAgentType());
    result.current.mutate({agentTypeId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should invalidate single agent-type and list queries on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result, queryClient} = renderHook(() => useUpdateAgentType());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({agentTypeId, data: mockRequest});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentTypeQueryKeys.AGENT_TYPE, agentTypeId]});
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]});
  });
});
