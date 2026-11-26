// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useGetSessions from '../useGetSessions';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
  };
});

describe('session APIs', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('lists a user’s sessions with paging', async () => {
    const page = {totalResults: 0, startIndex: 1, count: 0, sessions: []};
    mockHttpRequest.mockResolvedValue({data: page});

    const {result} = renderHook(() => useGetSessions({userId: 'user 1'}, 10, 20));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(page);
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/sessions?userId=user+1&limit=10&offset=20',
        method: 'GET',
      }),
    );
  });

  it('lists an application’s sessions', async () => {
    mockHttpRequest.mockResolvedValue({data: {totalResults: 0, startIndex: 1, count: 0, sessions: []}});

    const {result} = renderHook(() => useGetSessions({appId: 'app-1'}, 10, 0));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/sessions?appId=app-1&limit=10&offset=0'}),
    );
  });

  it('stops re-reading the list once listing is refused', async () => {
    vi.useFakeTimers({shouldAdvanceTime: true});
    try {
      mockHttpRequest.mockRejectedValue(Object.assign(new Error('forbidden'), {response: {status: 403}}));

      const {result} = renderHook(() => useGetSessions({userId: 'user-1'}, 10, 0));

      await waitFor(() => expect(result.current.isError).toBe(true));
      const calls = mockHttpRequest.mock.calls.length;
      await vi.advanceTimersByTimeAsync(60 * 1000);
      expect(mockHttpRequest).toHaveBeenCalledTimes(calls);
    } finally {
      vi.useRealTimers();
    }
  });
});
