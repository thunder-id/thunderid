// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {QueryClient} from '@tanstack/react-query';
import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import SessionQueryKeys from '../../constants/session-query-keys';
import useGetSessions from '../useGetSessions';
import useRevokeSession from '../useRevokeSession';
import useRevokeUserSessions from '../useRevokeUserSessions';

const mockHttpRequest = vi.fn();
const mockShowToast = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
    useToast: () => ({showToast: mockShowToast}),
  };
});

describe('session APIs', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockShowToast.mockReset();
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

  it('revokes one session and refreshes the lists', async () => {
    mockHttpRequest.mockResolvedValue({});
    const queryClient = new QueryClient();
    const invalidate = vi.spyOn(queryClient, 'invalidateQueries');

    const {result} = renderHook(() => useRevokeSession(), {queryClient});
    result.current.mutate('sess/1');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/sessions/sess%2F1', method: 'DELETE'}),
    );
    expect(invalidate).toHaveBeenCalledWith({queryKey: [SessionQueryKeys.SESSIONS]});
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('revokes all of a user’s sessions', async () => {
    mockHttpRequest.mockResolvedValue({});

    const {result} = renderHook(() => useRevokeUserSessions());
    result.current.mutate('user-1');

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://api.test.com/sessions?userId=user-1', method: 'DELETE'}),
    );
    expect(mockShowToast).toHaveBeenCalledWith(expect.any(String), 'success');
  });

  it('leaves a failed revoke to the caller', async () => {
    mockHttpRequest.mockRejectedValue(new Error('boom'));

    const {result} = renderHook(() => useRevokeSession());
    result.current.mutate('sess-1');

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(mockShowToast).not.toHaveBeenCalled();
  });
});
