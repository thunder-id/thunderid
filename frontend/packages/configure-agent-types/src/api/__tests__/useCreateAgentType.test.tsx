// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentTypeQueryKeys from '../../constants/agentTypeQueryKeys';
import type {ApiAgentType} from '../../models/agent-type';
import type {CreateAgentTypeRequest} from '../../models/requests';
import useCreateAgentType from '../useCreateAgentType';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
const mockShowToast = vi.fn();

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
    useToast: () => ({showToast: mockShowToast}),
  };
});

describe('useCreateAgentType', () => {
  const ouId = '111e8400-e29b-41d4-a716-446655440000';

  const mockRequest: CreateAgentTypeRequest = {
    handle: 'assistant',
    displayName: 'Assistant',
    ouId,
    systemAttributes: {display: 'name'},
    schema: {name: {type: 'string', required: true}},
  };

  const mockAgentType: ApiAgentType = {id: 'aaa-bbb-ccc', ...mockRequest};

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useCreateAgentType());

    expect(result.current.isIdle).toBe(true);
    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
  });

  it('should post the agent type and return the created one', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result} = renderHook(() => useCreateAgentType());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data).toEqual(mockAgentType);
    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: 'https://api.test.com/agent-types',
      method: 'POST',
      headers: {'Content-Type': 'application/json'},
      data: mockRequest,
    });
  });

  it('should invalidate the agent type lists and confirm with a toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result, queryClient} = renderHook(() => useCreateAgentType());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  // The caller renders the failure next to its form, so the hook must not also toast it.
  it('should expose the error without showing a toast when the organization unit already has a type', async () => {
    const apiError = Object.assign(new Error('Request failed'), {response: {data: {code: 'USRS-1021'}}});
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useCreateAgentType());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should use the server URL from the config', async () => {
    mockGetServerUrl.mockReturnValue('https://other.test.com');
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentType});

    const {result} = renderHook(() => useCreateAgentType());
    result.current.mutate(mockRequest);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: 'https://other.test.com/agent-types'}));
  });
});
