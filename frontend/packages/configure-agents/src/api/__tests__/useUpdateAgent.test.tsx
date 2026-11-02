// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentQueryKeys from '../../constants/agent-query-keys';
import type {Agent, UpdateAgentRequest} from '../../models/agent';
import useUpdateAgent from '../useUpdateAgent';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
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
    display: 'Updated Service',
    description: 'Updated description',
  };

  const mockRequest: UpdateAgentRequest = {
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
