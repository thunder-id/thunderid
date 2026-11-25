// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useAddUserToRoles, {AddUserToRolesError} from '../useAddUserToRoles';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => {
  const actual = await importOriginal<typeof import('@thunderid/contexts')>();
  return {...actual, useConfig: () => ({getServerUrl: () => 'https://api.test.com'})};
});

interface RecordedRequest {
  url: string;
  method: string;
  data?: unknown;
}

function requests(): RecordedRequest[] {
  return mockHttpRequest.mock.calls.map(([config]) => config as RecordedRequest);
}

describe('useAddUserToRoles', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it("assigns the user to each role through the role's assignments", async () => {
    mockHttpRequest.mockResolvedValue({data: undefined});
    const {result} = renderHook(() => useAddUserToRoles());

    result.current.mutate({userId: 'user-1', roleIds: ['admin', 'auditor']});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(requests()).toEqual([
      expect.objectContaining({
        url: 'https://api.test.com/roles/admin/assignments/add',
        method: 'POST',
        data: {assignments: [{id: 'user-1', type: 'user'}]},
      }),
      expect.objectContaining({
        url: 'https://api.test.com/roles/auditor/assignments/add',
        method: 'POST',
        data: {assignments: [{id: 'user-1', type: 'user'}]},
      }),
    ]);
  });

  it('tries every role and names those it could not assign, by ID when a role cannot be read', async () => {
    mockHttpRequest.mockImplementation((config: RecordedRequest) => {
      if (config.url.endsWith('/admin/assignments/add') || config.url.endsWith('/gone/assignments/add')) {
        return Promise.reject(new Error('forbidden'));
      }
      if (config.method === 'GET' && config.url.endsWith('/roles/admin')) {
        return Promise.resolve({data: {name: 'Administrator'}});
      }
      if (config.method === 'GET') {
        return Promise.reject(new Error('not found'));
      }
      return Promise.resolve({data: undefined});
    });
    const {result} = renderHook(() => useAddUserToRoles());

    result.current.mutate({userId: 'user-1', roleIds: ['admin', 'auditor', 'gone']});

    await waitFor(() => expect(result.current.isError).toBe(true));
    expect(requests().filter((request) => request.method === 'POST')).toHaveLength(3);
    expect(result.current.error).toBeInstanceOf(AddUserToRolesError);
    expect(result.current.error?.failedRoles).toEqual([
      {id: 'admin', name: 'Administrator'},
      {id: 'gone', name: 'gone'},
    ]);
  });
});
