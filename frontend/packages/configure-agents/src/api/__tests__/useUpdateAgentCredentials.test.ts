// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, act, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import useUpdateAgentCredentials, {type UpdateAgentCredentialsVariables} from '../useUpdateAgentCredentials';

const mockHttpRequest = vi.fn();
const mockGetServerUrl = vi.fn().mockReturnValue('https://api.test.com');
const mockShowToast = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({
      getServerUrl: mockGetServerUrl,
    }),
    useToast: () => ({
      showToast: mockShowToast,
    }),
  };
});

describe('useUpdateAgentCredentials', () => {
  const variables: UpdateAgentCredentialsVariables = {
    agentId: 'agent-1',
    data: {credentials: {password: 'new-password'}},
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReset().mockReturnValue('https://api.test.com');
    mockShowToast.mockReset();
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('should initialize with idle state', () => {
    const {result} = renderHook(() => useUpdateAgentCredentials());

    expect(result.current.data).toBeUndefined();
    expect(result.current.error).toBeNull();
    expect(result.current.isPending).toBe(false);
    expect(result.current.isIdle).toBe(true);
  });

  it('should post the credentials to the agent update-credentials endpoint', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useUpdateAgentCredentials());
    result.current.mutate(variables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/agents/agent-1/update-credentials',
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: variables.data,
      }),
    );
  });

  it('should encode the agent ID in the URL', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useUpdateAgentCredentials());
    result.current.mutate({...variables, agentId: 'a/b'});

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/agents/a%2Fb/update-credentials'}),
    );
  });

  it('should show a success toast on success', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useUpdateAgentCredentials());
    result.current.mutate(variables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('should not show a toast on error', async () => {
    mockHttpRequest.mockRejectedValueOnce(new Error('Failed to update credentials'));

    const {result} = renderHook(() => useUpdateAgentCredentials());
    result.current.mutate(variables);

    await waitFor(() => {
      expect(result.current.isError).toBe(true);
    });

    expect(mockShowToast).not.toHaveBeenCalled();
  });

  it('should reset the mutation state', async () => {
    mockHttpRequest.mockResolvedValueOnce(undefined);

    const {result} = renderHook(() => useUpdateAgentCredentials());
    result.current.mutate(variables);

    await waitFor(() => {
      expect(result.current.isSuccess).toBe(true);
    });

    act(() => {
      result.current.reset();
    });

    await waitFor(() => {
      expect(result.current.isIdle).toBe(true);
    });
  });
});
