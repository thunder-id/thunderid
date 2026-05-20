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
import useDeleteAgent from '../useDeleteAgent';

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

describe('useDeleteAgent', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  const agentId = '550e8400-e29b-41d4-a716-446655440000';

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

  it('should successfully delete an agent', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteAgent());
    result.current.mutate(agentId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `https://api.test.com/agents/${agentId}`,
      method: 'DELETE',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('should handle API error', async () => {
    const apiError = new Error('Delete failed');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteAgent());
    result.current.mutate(agentId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should invalidate agents list query on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteAgent());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(agentId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENTS]});
  });

  it('should remove the deleted agent from the cache on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteAgent());
    const removeQueriesSpy = vi.spyOn(queryClient, 'removeQueries');

    result.current.mutate(agentId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(removeQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENT, agentId]});
  });
});
