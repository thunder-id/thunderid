// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {waitFor, renderHook} from '@thunderid/test-utils';
import {describe, it, expect, beforeEach, vi} from 'vitest';
import useRemoveUserFromRole from '../useRemoveUserFromRole';

const mockHttpRequest = vi.fn();

vi.mock('@thunderid/react', () => ({
  useThunderID: () => ({http: {request: mockHttpRequest}}),
}));

vi.mock('@thunderid/contexts', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@thunderid/contexts')>()),
  useConfig: () => ({getServerUrl: () => 'https://api.test.com'}),
}));

describe('useRemoveUserFromRole', () => {
  beforeEach(() => {
    mockHttpRequest.mockReset();
  });

  it("removes the user through the role's assignments", async () => {
    mockHttpRequest.mockResolvedValue({data: undefined});
    const {result} = renderHook(() => useRemoveUserFromRole());

    result.current.mutate({userId: 'user-1', roleId: 'role-1'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({
        url: 'https://api.test.com/roles/role-1/assignments/remove',
        method: 'POST',
        data: {assignments: [{id: 'user-1', type: 'user'}]},
      }),
    );
  });

  it('names the organization unit the assignment is made in', async () => {
    mockHttpRequest.mockResolvedValue({data: undefined});
    const {result} = renderHook(() => useRemoveUserFromRole());

    result.current.mutate({userId: 'user-1', roleId: 'role-1', ouId: 'ou-child'});

    await waitFor(() => expect(result.current.isSuccess).toBe(true));
    expect(mockHttpRequest).toHaveBeenCalledWith(
      expect.objectContaining({data: {ouId: 'ou-child', assignments: [{id: 'user-1', type: 'user'}]}}),
    );
  });

  it('fails when the role cannot be removed', async () => {
    mockHttpRequest.mockRejectedValue(new Error('forbidden'));
    const {result} = renderHook(() => useRemoveUserFromRole());

    result.current.mutate({userId: 'user-1', roleId: 'role-1'});

    await waitFor(() => expect(result.current.isError).toBe(true));
  });
});
