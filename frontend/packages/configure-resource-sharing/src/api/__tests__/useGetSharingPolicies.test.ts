// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {SharingPolicy} from '../../models/sharing-policy';
import useGetSharingPolicies from '../useGetSharingPolicies';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

const policy = (id: string): SharingPolicy => ({id, stage: 'reshare', version: 1});

describe('useGetSharingPolicies', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('follows pagination until every policy is read', async () => {
    mockHttpRequest
      .mockResolvedValueOnce({data: {totalResults: 3, startIndex: 1, count: 2, policies: [policy('a'), policy('b')]}})
      .mockResolvedValueOnce({data: {totalResults: 3, startIndex: 3, count: 1, policies: [policy('c')]}});

    const {result} = renderHook(() => useGetSharingPolicies('applications/app-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.map((p) => p.id)).toEqual(['a', 'b', 'c']);
    expect(mockHttpRequest).toHaveBeenNthCalledWith(
      2,
      expect.objectContaining({url: 'https://localhost:8090/applications/app-1/sharing-policies?offset=2'}),
    );
  });

  it('stops at an empty page even if the total says otherwise', async () => {
    mockHttpRequest.mockResolvedValueOnce({data: {totalResults: 5, startIndex: 1, count: 0, policies: []}});

    const {result} = renderHook(() => useGetSharingPolicies('applications/app-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('does not fetch when disabled', () => {
    renderHook(() => useGetSharingPolicies('applications/app-1', false));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });

  it('exposes a failed read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('down'));

    const {result} = renderHook(() => useGetSharingPolicies('applications/app-1'));

    await waitFor(() => expect(result.current.isError).toBe(true), {timeout: 5000});
  });
});
