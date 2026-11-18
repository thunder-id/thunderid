// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import type {UserRole} from '../../models/users';
import useGetUserRoles from '../useGetUserRoles';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
}));

const editor: UserRole = {id: 'role-editor', name: 'Editor', ouId: 'ou-1'};
const viewer: UserRole = {id: 'role-viewer', name: 'Viewer', ouId: 'ou-1'};
const auditor: UserRole = {id: 'role-auditor', name: 'Auditor', ouId: 'ou-1'};

// The user IDs assigned each role, in the order the API pages through them.
let assignees: Record<string, {id: string; ouId?: string; isReadOnly?: boolean}[]> = {};

function urls(): URL[] {
  return mockHttpRequest.mock.calls.map(([config]) => new URL((config as {url: string}).url));
}

describe('useGetUserRoles', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
    assignees = {};
    mockHttpRequest.mockImplementation((config: {url: string}) => {
      const {pathname, searchParams} = new URL(config.url);
      const all = assignees[pathname.split('/')[2]] ?? [];
      const offset = Number(searchParams.get('offset'));
      const limit = Number(searchParams.get('limit'));
      return Promise.resolve({
        data: {totalResults: all.length, assignments: all.slice(offset, offset + limit)},
      });
    });
  });

  it("finds the roles assigned to the user among each role's user assignments", async () => {
    assignees = {
      'role-editor': [{id: 'user-2'}, {id: 'user-1'}],
      'role-viewer': [{id: 'user-2'}],
      'role-auditor': [{id: 'user-1', isReadOnly: true}],
    };

    const {result} = renderHook(() => useGetUserRoles('user-1', [editor, viewer, auditor]));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([
      {...editor, isReadOnly: false},
      {...auditor, isReadOnly: true},
    ]);
    expect(urls().map((url) => url.pathname)).toEqual([
      '/roles/role-editor/assignments',
      '/roles/role-viewer/assignments',
      '/roles/role-auditor/assignments',
    ]);
    expect(urls()[0].searchParams.get('type')).toBe('user');
    expect(urls()[0].searchParams.get('limit')).toBe('100');
  });

  it('keeps the organization unit each assignment is made in', async () => {
    assignees = {'role-editor': [{id: 'user-1', ouId: 'ou-child'}]};

    const {result} = renderHook(() => useGetUserRoles('user-1', [editor]));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([{...editor, isReadOnly: false, assignmentOuId: 'ou-child'}]);
  });

  it('reads further pages of a role until it finds the user', async () => {
    assignees = {
      'role-editor': [...Array.from({length: 150}, (_, index) => ({id: `other-${index}`})), {id: 'user-1'}],
    };

    const {result} = renderHook(() => useGetUserRoles('user-1', [editor]));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([{...editor, isReadOnly: false}]);
    expect(urls().map((url) => url.searchParams.get('offset'))).toEqual(['0', '100']);
  });

  it('stops at the last page of a role the user does not hold', async () => {
    assignees = {'role-editor': Array.from({length: 150}, (_, index) => ({id: `other-${index}`}))};

    const {result} = renderHook(() => useGetUserRoles('user-1', [editor]));

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(result.current.data).toEqual([]);
    expect(urls()).toHaveLength(2);
  });

  it('fails when a role cannot be read', async () => {
    mockHttpRequest.mockRejectedValue(new Error('forbidden'));

    const {result} = renderHook(() => useGetUserRoles('user-1', [editor]));

    await waitFor(() => expect(result.current.isError).toBe(true));
  });

  it('reads nothing until the roles are given, or without a user ID', () => {
    renderHook(() => useGetUserRoles('user-1', undefined));
    renderHook(() => useGetUserRoles('', [editor]));

    expect(mockHttpRequest).not.toHaveBeenCalled();
  });
});
