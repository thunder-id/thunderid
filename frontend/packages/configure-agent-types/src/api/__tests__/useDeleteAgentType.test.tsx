// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentTypeQueryKeys from '../../constants/agentTypeQueryKeys';
import useDeleteAgentType from '../useDeleteAgentType';
import useGetAgentType from '../useGetAgentType';

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

describe('useDeleteAgentType', () => {
  const agentTypeId = 'aaa-bbb-ccc';

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useDeleteAgentType());

    expect(result.current.isIdle).toBe(true);
    expect(result.current.error).toBeNull();
  });

  it('should send a DELETE request for the agent type', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useDeleteAgentType());
    result.current.mutate(agentTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith({
      url: `https://api.test.com/agent-types/${agentTypeId}`,
      method: 'DELETE',
      headers: {'Content-Type': 'application/json'},
    });
  });

  it('should mark the detail stale, refresh the lists, and confirm with a toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result, queryClient} = renderHook(() => useDeleteAgentType());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate(agentTypeId);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({
      queryKey: [AgentTypeQueryKeys.AGENT_TYPE, agentTypeId],
      refetchType: 'none',
    });
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  // The agent type page keeps observing its detail query while the delete runs, so a delete must not
  // make React Query fetch the type it just deleted.
  it('should not fetch the deleted agent type again while its page still observes it', async () => {
    mockHttpRequest.mockImplementation((config: {method: string}) =>
      Promise.resolve(config.method === 'GET' ? {data: {id: agentTypeId, handle: 'assistant'}} : undefined),
    );

    const {result} = renderHook(() => ({detail: useGetAgentType(agentTypeId), remove: useDeleteAgentType()}));

    await waitFor(() => {
      expect(result.current.detail.isSuccess).toBe(true);
    });

    result.current.remove.mutate(agentTypeId);

    await waitFor(() => {
      expect(result.current.remove.isSuccess).toBe(true);
    });

    const methods = mockHttpRequest.mock.calls.map(([config]) => (config as {method: string}).method);
    expect(methods).toEqual(['GET', 'DELETE']);
  });

  it('should expose the error without showing a toast when agents still use the type', async () => {
    const apiError = Object.assign(new Error('Request failed'), {response: {data: {code: 'USRS-1022'}}});
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useDeleteAgentType());
    result.current.mutate(agentTypeId);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toBe(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
