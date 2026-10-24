// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook, act} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import usePagedOrganizationUnits from '../usePagedOrganizationUnits';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

const unit = (id: string): {id: string; handle: string; name: string} => ({id, handle: id, name: id});

describe('usePagedOrganizationUnits', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('loads the root units when no parent is given', async () => {
    mockHttpRequest.mockResolvedValue({
      data: {totalResults: 1, startIndex: 1, count: 1, organizationUnits: [unit('r')]},
    });

    const {result} = renderHook(() => usePagedOrganizationUnits(null));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/organization-units?limit=50&offset=0'}),
    );
    expect(result.current.hasNextPage).toBe(false);
  });

  it('loads the next page of a parent’s children on demand', async () => {
    mockHttpRequest
      .mockResolvedValueOnce({data: {totalResults: 2, startIndex: 1, count: 1, organizationUnits: [unit('a')]}})
      .mockResolvedValueOnce({data: {totalResults: 2, startIndex: 2, count: 1, organizationUnits: [unit('b')]}});

    const {result} = renderHook(() => usePagedOrganizationUnits('parent'));

    await waitFor(() => expect(result.current.hasNextPage).toBe(true));
    await act(async () => {
      await result.current.fetchNextPage();
    });

    expect(mockHttpRequest).toHaveBeenLastCalledWith(
      expect.objectContaining({url: 'https://localhost:8090/organization-units/parent/ous?limit=50&offset=1'}),
    );
    expect(result.current.data?.pages.flatMap((page) => page.organizationUnits.map((ou) => ou.id))).toEqual(['a', 'b']);
  });
});
