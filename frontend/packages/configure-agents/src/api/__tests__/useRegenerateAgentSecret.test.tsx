// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import AgentQueryKeys from '../../constants/agent-query-keys';
import type {Agent} from '../../models/agent';
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
    useToast: vi.fn().mockReturnValue({showToast: vi.fn()}),
  };
});

const {useThunderID} = await import('@thunderid/react');
const {useConfig} = await import('@thunderid/contexts');

describe('useRegenerateAgentSecret', () => {
  let mockHttpRequest: ReturnType<typeof vi.fn>;
  const agentId = '550e8400-e29b-41d4-a716-446655440000';

  const mockAgentWithOAuth: Agent = {
    id: agentId,
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Billing Service',
    inboundAuthConfig: [
      {
        type: 'oauth2',
        config: {
          clientId: 'agent-billing',
          clientSecret: 'old-secret',
          grantTypes: ['client_credentials'],
          tokenEndpointAuthMethod: 'client_secret_basic',
          responseTypes: [],
        },
      },
    ],
  };

  const mockAgentWithoutOAuth: Agent = {
    id: agentId,
    ouId: '111e8400-e29b-41d4-a716-446655440000',
    type: 'default',
    name: 'Entity-only agent',
    inboundAuthConfig: [],
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

  it('should fetch the agent then PUT with a freshly generated client secret', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentWithOAuth}).mockResolvedValueOnce({data: mockAgentWithOAuth});

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: `https://api.test.com/agents/${agentId}`,
        method: 'GET',
      }),
    );

    const putCall = mockHttpRequest.mock.calls[1][0] as {
      url: string;
      method: string;
      data: Agent;
    };
    expect(putCall.url).toBe(`https://api.test.com/agents/${agentId}`);
    expect(putCall.method).toBe('PUT');
    const oauthCfg = putCall.data.inboundAuthConfig?.[0]?.config;
    expect(oauthCfg?.clientSecret).toBeTruthy();
    expect(oauthCfg?.clientSecret).not.toBe('old-secret');
    // Base64url with no padding — 32 bytes encodes to 43 chars
    expect(oauthCfg?.clientSecret).toMatch(/^[A-Za-z0-9_-]{43}$/);

    // Stripped server-generated fields are not echoed back on PUT.
    expect((putCall.data as {id?: string}).id).toBeUndefined();
    expect((putCall.data as {clientId?: string}).clientId).toBeUndefined();

    expect(result.current.data?.clientSecret).toBe(oauthCfg?.clientSecret);
  });

  it('should reject when the agent has no OAuth2 configuration', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentWithoutOAuth});

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error?.message).toContain('OAuth2');
    // Only the GET fired; no PUT.
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('should propagate GET errors', async () => {
    const apiError = new Error('Agent not found');
    mockHttpRequest.mockRejectedValueOnce(apiError);

    const {result} = renderHook(() => useRegenerateAgentSecret());
    result.current.mutate({agentId});

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(result.current.error).toEqual(apiError);
  });

  it('should invalidate single agent and list queries on success', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: mockAgentWithOAuth}).mockResolvedValueOnce({data: mockAgentWithOAuth});

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
