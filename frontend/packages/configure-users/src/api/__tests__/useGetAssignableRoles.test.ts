// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {UserRoleListResponse} from '../../models/users';
import useGetAssignableRoles from '../useGetAssignableRoles';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
}));

describe('useGetAssignableRoles', () => {
  const response: UserRoleListResponse = {
    totalResults: 1,
    count: 1,
    roles: [{id: 'role-1', name: 'Editor', ouId: 'ou-1', projectId: 'project-1'}],
  };

  beforeEach(() => {
    mockHttpRequest.mockReset();
    mockHttpRequest.mockResolvedValue({data: response});
  });

  it("lists the project's roles when a project is given", async () => {
    const {result} = renderHook(() => useGetAssignableRoles('project-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual(response);

    const parsed = new URL((mockHttpRequest.mock.calls[0][0] as {url: string}).url);
    expect(parsed.origin + parsed.pathname).toBe('https://api.test.com/roles');
    expect(parsed.searchParams.get('projectId')).toBe('project-1');
    expect(parsed.searchParams.get('limit')).toBe('100');
  });

  it('lists every role when no project is given', async () => {
    const {result} = renderHook(() => useGetAssignableRoles(undefined));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));

    const parsed = new URL((mockHttpRequest.mock.calls[0][0] as {url: string}).url);
    expect(parsed.searchParams.has('projectId')).toBe(false);
  });

  it('reads every page of roles', async () => {
    const page = (start: number, size: number): UserRoleListResponse['roles'] =>
      Array.from({length: size}, (_, index) => ({
        id: `role-${start + index}`,
        name: `Role ${start + index}`,
        ouId: 'ou-1',
      }));
    mockHttpRequest.mockImplementation((config: {url: string}) => {
      const offset = Number(new URL(config.url).searchParams.get('offset'));
      const roles = offset === 0 ? page(0, 100) : page(100, 20);
      return Promise.resolve({data: {totalResults: 120, count: roles.length, roles}});
    });

    const {result} = renderHook(() => useGetAssignableRoles('project-1'));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.roles).toHaveLength(120);
    expect(result.current.data?.count).toBe(120);
    expect(result.current.data?.totalResults).toBe(120);
    expect(
      mockHttpRequest.mock.calls.map(([config]) => new URL((config as {url: string}).url).searchParams.get('offset')),
    ).toEqual(['0', '100']);
  });

  it('stops when a page comes back empty, whatever the total says', async () => {
    mockHttpRequest.mockResolvedValue({data: {totalResults: 500, count: 0, roles: []}});

    const {result} = renderHook(() => useGetAssignableRoles(undefined));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data?.roles).toEqual([]);
    expect(mockHttpRequest).toHaveBeenCalledTimes(1);
  });

  it('does not fetch while disabled', () => {
    renderHook(() => useGetAssignableRoles('project-1', false));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
