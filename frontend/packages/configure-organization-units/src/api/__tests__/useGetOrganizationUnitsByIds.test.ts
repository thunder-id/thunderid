// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useGetOrganizationUnitsByIds from '../useGetOrganizationUnitsByIds';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

describe('useGetOrganizationUnitsByIds', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it('fetches each unit once and maps it by id', async () => {
    mockHttpRequest.mockImplementation(({url}: {url: string}) => {
      const id = url.split('/').pop()!;
      return Promise.resolve({data: {id, handle: id, name: `Unit ${id}`}});
    });

    const {result} = renderHook(() => useGetOrganizationUnitsByIds(['a', 'b', 'a']));

    await waitFor(() => expect(result.current.size).toBe(2));
    expect(result.current.get('a')?.name).toBe('Unit a');
    expect(mockHttpRequest).toHaveBeenCalledTimes(2);
  });

  it('leaves out units that fail to load', async () => {
    mockHttpRequest.mockImplementation(({url}: {url: string}) =>
      url.endsWith('/bad')
        ? Promise.reject(new Error('missing'))
        : Promise.resolve({data: {id: 'good', handle: 'good', name: 'Good'}}),
    );

    const {result} = renderHook(() => useGetOrganizationUnitsByIds(['good', 'bad']));

    await waitFor(() => expect(result.current.get('good')?.name).toBe('Good'));
    expect(result.current.has('bad')).toBe(false);
  });
});
