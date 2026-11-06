// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useGetMandatableRoles from '../useGetMandatableRoles';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

// A role listed for a unit carries that unit's ID whoever owns it, as the roles API returns it.
const role = (id: string, listedOuId: string, isOwnedByOU: boolean) => ({
  id,
  name: `${id}-name`,
  ouId: listedOuId,
  isOwnedByOU,
});

const listUrl = (ouId: string, offset = 0) =>
  `https://localhost:8090/roles?${new URLSearchParams({filter: `ouId eq "${ouId}"`, limit: '100', offset: String(offset)}).toString()}`;

// Answers each listing by the unit its filter names.
const answerBy = (pages: Record<string, {totalResults: number; roles: ReturnType<typeof role>[]}[]>) => {
  mockHttpRequest.mockImplementation(({url}: {url: string}) => {
    const params = new URL(url).searchParams;
    const ouId = /ouId eq "([^"]+)"/.exec(params.get('filter') ?? '')?.[1] ?? '';
    const offset = Number(params.get('offset'));
    const queue = pages[ouId] ?? [{totalResults: 0, roles: []}];
    const page = queue.find((p, i) => queue.slice(0, i).reduce((n, q) => n + q.roles.length, 0) === offset);
    return Promise.resolve({data: page ?? {totalResults: 0, roles: []}});
  });
};

describe('useGetMandatableRoles', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it("keeps the issuer's own roles shared with the unit, read against the issuer's listing", async () => {
    answerBy({
      retail: [
        {
          totalResults: 4,
          roles: [
            role('own-by-unit', 'retail', true),
            role('from-issuer', 'retail', false),
            role('from-other', 'retail', false),
            role('from-issuer-2', 'retail', false),
          ],
        },
      ],
      acme: [
        {
          totalResults: 3,
          roles: [
            role('from-issuer', 'acme', true),
            role('from-issuer-2', 'acme', true),
            role('from-root', 'acme', false),
          ],
        },
      ],
    });

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates.map((r) => r.id)).toEqual(['from-issuer', 'from-issuer-2']);
    expect(result.current.data?.candidates[0]).toEqual({
      id: 'from-issuer',
      name: 'from-issuer-name',
      description: undefined,
      ouId: 'acme',
    });
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('retail')}));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('acme')}));
  });

  it('names every role the unit sees, so a mandated role that is no longer a candidate is not shown by ID', async () => {
    answerBy({
      retail: [{totalResults: 2, roles: [role('from-other', 'retail', false), role('own-by-unit', 'retail', true)]}],
      acme: [{totalResults: 0, roles: []}],
    });

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates).toEqual([]);
    expect(result.current.data?.names).toEqual({'from-other': 'from-other-name', 'own-by-unit': 'own-by-unit-name'});
  });

  it('follows pagination to the end', async () => {
    const first = Array.from({length: 100}, (_, i) => role(`r${i}`, 'retail', false));
    answerBy({
      retail: [
        {totalResults: 101, roles: first},
        {totalResults: 101, roles: [role('last', 'retail', false)]},
      ],
      acme: [
        {totalResults: 101, roles: Array.from({length: 100}, (_, i) => role(`r${i}`, 'acme', true))},
        {totalResults: 101, roles: [role('last', 'acme', true)]},
      ],
    });

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates).toHaveLength(101);
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('retail', 100)}));
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('acme', 100)}));
  });

  it('does not fetch without a unit or an issuer', () => {
    renderHook(() => useGetMandatableRoles('', 'acme'));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
