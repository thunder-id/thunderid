// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {OwnedAgentListResponse} from '../../models/users';
import useGetUserOwnedAgents from '../useGetUserOwnedAgents';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {
      request: mockHttpRequest,
    },
  }),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({
    getServerUrl: () => 'https://api.test.com',
  }),
}));

describe('useGetUserOwnedAgents', () => {
  const response: OwnedAgentListResponse = {
    totalResults: 1,
    count: 1,
    agents: [{id: 'agent-1', display: 'Claude Code', kind: 'personal-assistant'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('lists the agents filtered by the user as owner', async () => {
    mockHttpRequest.mockResolvedValue({data: response});

    const {result} = renderHook(() => useGetUserOwnedAgents('user-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(response);

    const {url} = mockHttpRequest.mock.calls[0][0] as {url: string};
    const parsed = new URL(url);
    expect(parsed.origin + parsed.pathname).toBe('https://api.test.com/agents');
    expect(parsed.searchParams.get('filter')).toBe('owner eq "user-1"');
    expect(parsed.searchParams.get('limit')).toBe('100');
    expect(parsed.searchParams.get('include')).toBe('display');
  });

  it('does not fetch without a user ID', () => {
    renderHook(() => useGetUserOwnedAgents(''));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
