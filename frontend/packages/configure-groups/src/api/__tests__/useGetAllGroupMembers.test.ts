// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor} from '@testing-library/react';
import {renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, afterEach, vi} from 'vitest';
import type {MemberListResponse} from '../../models/group';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({
    http: {request: mockHttpRequest},
  }),
}));

const mockGetServerUrl = vi.fn<() => string>(() => 'https://localhost:8090');
vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: mockGetServerUrl}),
  };
});

const {default: useGetAllGroupMembers} = await import('../useGetAllGroupMembers');

describe('useGetAllGroupMembers', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockGetServerUrl.mockReturnValue('https://localhost:8090');
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  it('fetches and combines every member page', async () => {
    const firstPage: MemberListResponse = {
      totalResults: 101,
      startIndex: 1,
      count: 100,
      members: Array.from({length: 100}, (_, index) => ({id: `member-${index}`, type: 'user' as const})),
    };
    const secondPage: MemberListResponse = {
      totalResults: 101,
      startIndex: 101,
      count: 1,
      members: [{id: 'member-100', type: 'user'}],
    };
    mockHttpRequest.mockResolvedValueOnce({data: firstPage}).mockResolvedValueOnce({data: secondPage});

    const {result} = renderHook(() => useGetAllGroupMembers('g1'));

    await waitFor(() => {
      expect(result.current.data?.members).toHaveLength(101);
    });

    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      1,
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1/members?limit=100&offset=0&include=display',
      }),
    );
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({
        url: 'https://localhost:8090/groups/g1/members?limit=100&offset=100&include=display',
      }),
    );
  });

  it('does not fetch when groupId is undefined', () => {
    const {result} = renderHook(() => useGetAllGroupMembers(undefined));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
