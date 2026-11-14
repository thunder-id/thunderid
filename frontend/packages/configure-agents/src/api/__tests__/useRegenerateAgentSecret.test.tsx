// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentQueryKeys from '../../constants/agent-query-keys';
import useRegenerateAgentSecret from '../useRegenerateAgentSecret';

vi.mock('@thunderid/react', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/react')>()),
  useThunderID: vi.fn(),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: vi.fn(),
    useToast: vi.fn(),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig, useToast} = await import('@thunderid/contexts');

describe('useRegenerateAgentSecret', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  let mockShowToast: ReturnType<typeof vi.fn>;
  const agentId = '550e8400-e29b-41d4-a716-446655440000';

  beforeEach(() => {
    mockHttpRequest = vi.fn();
    mockShowToast = vi.fn();

    vi.mocked(useThunderID).mockReturnValue({
      http: {request: mockHttpRequest},
    } as unknown as ReturnType<typeof useThunderID>);

    vi.mocked(useConfig).mockReturnValue({
      getServerUrl: () => 'https://api.test.com',
    } as ReturnType<typeof useConfig>);

    vi.mocked(useToast).mockReturnValue({showToast: mockShowToast} as unknown as ReturnType<typeof useToast>);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('asks the server to regenerate the secret and returns the one it generated', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {clientSecret: 'server-generated'}});

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: `https://api.test.com/agents/${agentId}/client-secret/regenerate`,
        method: 'POST',
      }),
    );
    expect(result.current.data).toEqual({clientSecret: 'server-generated'});
    expect(mockShowToast).toHaveBeenCalledWith('Client secret regenerated successfully', 'success');
  });

  it('warns when the tokens issued under the old secret may not be revoked', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {clientSecret: 'server-generated', revocationIncomplete: true}});

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(result.current.data?.revocationIncomplete).toBe(true);
    expect(mockShowToast).toHaveBeenCalledWith(expect.stringContaining('Regenerate it again'), 'warning');
  });

  it('propagates a refusal', async () => {
    const apiError = new Error('Agent has no client secret');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('invalidates the agent and the agent list on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {clientSecret: 'server-generated'}});

    const {result, queryClient} = renderHook(() => useRegenerateAgentSecret());
    const invalidateQueriesSpy = vi.spyOn(queryClient, 'invalidateQueries');

    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENT, agentId]});
    expect(invalidateQueriesSpy).toHaveBeenCalledWith({queryKey: [AgentQueryKeys.AGENTS]});
  });
});
