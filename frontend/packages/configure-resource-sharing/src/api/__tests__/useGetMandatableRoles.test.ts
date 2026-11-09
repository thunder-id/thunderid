// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {SharingPolicy} from '../../models/sharing-policy';
import useGetMandatableRoles from '../useGetMandatableRoles';

const mockHttpRequest = vi.fn();
vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://localhost:8090'})};
});

const role = (id: string, isOwnedByOU: boolean) => ({id, name: `${id}-name`, ouId: 'acme', isOwnedByOU});

const listUrl = (ouId: string, offset = 0) =>
  `https://localhost:8090/roles?${new URLSearchParams({filter: `ouId eq "${ouId}"`, limit: '100', offset: String(offset)}).toString()}`;

const policiesUrl = (roleId: string) => `https://localhost:8090/roles/${roleId}/sharing-policies?offset=0`;

const sharedWith = (initiatingOuId: string, ...units: string[]): SharingPolicy => ({
  id: `policy-${initiatingOuId}`,
  stage: 'share',
  version: 1,
  initiatingOuId,
  childUnits: {mode: 'selected', units: units.map((ouId) => ({ouId, reach: 'unit'}))},
});

// Answers the issuer's role listing page by page, and each role's policies by its ID.
const answer = (
  issuerPages: {totalResults: number; roles: ReturnType<typeof role>[]}[],
  policies: Record<string, SharingPolicy[]>,
) => {
  mockHttpRequest.mockImplementation(({url}: {url: string}) => {
    const policyMatch = /\/roles\/([^/]+)\/sharing-policies/.exec(url);
    if (policyMatch) {
      const list = policies[policyMatch[1]] ?? [];
      return Promise.resolve({data: {totalResults: list.length, policies: list}});
    }
    const offset = Number(new URL(url).searchParams.get('offset'));
    const page = issuerPages.find((_, i) => issuerPages.slice(0, i).reduce((n, p) => n + p.roles.length, 0) === offset);
    return Promise.resolve({data: page ?? {totalResults: 0, roles: []}});
  });
};

describe('useGetMandatableRoles', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it("keeps the issuer's own roles its policy shares with the unit, without listing the unit's roles", async () => {
    answer(
      [
        {
          totalResults: 4,
          roles: [
            role('to-retail', true),
            role('to-other', true),
            role('from-root', false),
            role('reshared-by-retail', true),
          ],
        },
      ],
      {
        'to-retail': [sharedWith('acme', 'retail')],
        'to-other': [sharedWith('acme', 'wholesale')],
        'from-root': [sharedWith('root', 'retail')],
        'reshared-by-retail': [sharedWith('retail', 'retail-east')],
      },
    );

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates).toEqual([
      {id: 'to-retail', name: 'to-retail-name', description: undefined, ouId: 'acme'},
    ]);
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('acme')}));
    expect(mockHttpRequest).not.toHaveBeenCalledWith(expect.objectContaining({url: listUrl('retail')}));
    expect(mockHttpRequest).not.toHaveBeenCalledWith(expect.objectContaining({url: policiesUrl('from-root')}));
  });

  it('names every role the issuer sees, so a mandated role that is no longer a candidate is not shown by ID', async () => {
    answer([{totalResults: 2, roles: [role('unshared', true), role('from-root', false)]}], {});

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates).toEqual([]);
    expect(result.current.data?.names).toEqual({unshared: 'unshared-name', 'from-root': 'from-root-name'});
  });

  it("follows pagination of the issuer's roles to the end", async () => {
    answer(
      [
        {totalResults: 101, roles: Array.from({length: 100}, (_, i) => role(`r${i}`, true))},
        {totalResults: 101, roles: [role('last', true)]},
      ],
      {last: [sharedWith('acme', 'retail')]},
    );

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.candidates.map((r) => r.id)).toEqual(['last']);
    expect(mockHttpRequest).toHaveBeenCalledWith(expect.objectContaining({url: listUrl('acme', 100)}));
  });

  it('fails when a role policy cannot be read', async () => {
    mockHttpRequest.mockImplementation(({url}: {url: string}) =>
      url.includes('/sharing-policies')
        ? Promise.reject(new Error('denied'))
        : Promise.resolve({data: {totalResults: 1, roles: [role('to-retail', true)]}}),
    );

    const {result} = renderHook(() => useGetMandatableRoles('retail', 'acme'));

    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it('does not fetch without a unit or an issuer', () => {
    renderHook(() => useGetMandatableRoles('', 'acme'));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
