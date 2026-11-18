// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import UserQueryKeys from '../constants/user-query-keys';

/** Variables for taking one role from a user. */
export interface RemoveUserFromRoleVariables {
  userId: string;
  roleId: string;
  /**
   * The organization unit the assignment is made in. Without it the server takes the assignment from the
   * organization unit that owns the role.
   */
  ouId?: string;
}

/**
 * Custom hook to take a role from a user.
 *
 * It posts to `/roles/{id}/assignments/remove` itself: `configure-roles` depends on this package, so its
 * `useRemoveRoleAssignment` cannot be used here.
 *
 * @returns TanStack Query mutation object for taking the role from the user
 */
export default function useRemoveUserFromRole(): UseMutationResult<void, Error, RemoveUserFromRoleVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<void, Error, RemoveUserFromRoleVariables>({
    mutationFn: async ({userId, roleId, ouId}: RemoveUserFromRoleVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/roles/${encodeURIComponent(roleId)}/assignments/remove`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: {...(ouId ? {ouId} : {}), assignments: [{id: userId, type: 'user'}]},
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSettled: (_data, _error, {userId}) => {
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USER_ROLES, userId]}).catch(() => null);
    },
  });
}
