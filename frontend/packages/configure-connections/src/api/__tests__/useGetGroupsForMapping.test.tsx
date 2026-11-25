// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {renderHook, waitFor} from '@thunderid/test-utils';
import {beforeEach, describe, expect, it, vi} from 'vitest';
import useGetGroupsForMapping from '../useGetGroupsForMapping';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {
    ...actual,
    useConfig: () => ({getServerUrl: () => 'http://localhost:8090'}),
  };
});

describe('useGetGroupsForMapping', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset().mockResolvedValue({data: {groups: [{id: 'group-1', name: 'Admins'}]}});
  });

  it("lists groups from the connection's organization unit", async () => {
    const {result} = renderHook(() => useGetGroupsForMapping({limit: 100, ouId: 'ou-1'}));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    const url = new URL((mockHttpRequest.mock.calls[0][0] as {url: string}).url);
    expect(url.pathname).toBe('/groups');
    expect(url.searchParams.get('filter')).toBe('ouId eq "ou-1"');
    expect(url.searchParams.get('limit')).toBe('100');
    expect(result.current.data?.groups).toEqual([{id: 'group-1', name: 'Admins'}]);
  });

  it('lists nothing until the organization unit is known', () => {
    const {result} = renderHook(() => useGetGroupsForMapping({limit: 100}));

    expect(result.current.fetchStatus).toBe('idle');
    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
