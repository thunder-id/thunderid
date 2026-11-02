// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
    handle: 'default',
    displayName: 'Default',
    ouId,
    schema: {model: {type: 'string', required: false}},
  };

  const mockRequest: UpdateAgentTypeRequest = {
    displayName: 'Default',
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
