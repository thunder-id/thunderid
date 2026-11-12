// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';

/** Variables for giving one user roles. */
export interface AddUserToRolesVariables {
  userId: string;
  roleIds: string[];
}

/** A role the user could not be given, with its name when it could be read. */
export interface FailedRole {
  id: string;
  name: string;
}

/** Error raised when the user could not be given one or more of the roles. */
export class AddUserToRolesError extends Error {
  readonly failedRoles: FailedRole[];

  constructor(failedRoles: FailedRole[]) {
    super(`Could not give the user the roles ${failedRoles.map((role) => role.name).join(', ')}`);
    this.name = 'AddUserToRolesError';
    this.failedRoles = failedRoles;
  }
}

/**
 * Custom hook to give a user roles, one assignment request per role. Every role is attempted; should any
 * fail, the mutation fails with an {@link AddUserToRolesError} naming those roles.
 *
 * It posts to `/roles/{id}/assignments/add` itself: `configure-roles` depends on this package, so its
 * `useAddRoleAssignments` cannot be used here.
 *
 * @returns TanStack Query mutation object for giving the user the roles
 */
export default function useAddUserToRoles(): UseMutationResult<void, AddUserToRolesError, AddUserToRolesVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<void, AddUserToRolesError, AddUserToRolesVariables>({
    mutationFn: async ({userId, roleIds}: AddUserToRolesVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();
      const failedRoleIds: string[] = [];
      for (const roleId of roleIds) {
        try {
          await http.request({
            url: `${serverUrl}/roles/${encodeURIComponent(roleId)}/assignments/add`,
            method: 'POST',
            headers: {
              'Content-Type': 'application/json',
            },
            data: {assignments: [{id: userId, type: 'user'}]},
          } as unknown as Parameters<typeof http.request>[0]);
        } catch {
          failedRoleIds.push(roleId);
        }
      }
      if (failedRoleIds.length === 0) {
        return;
      }

      // The roles are named where they can be read, and by ID otherwise.
      const failedRoles: FailedRole[] = await Promise.all(
        failedRoleIds.map(async (roleId): Promise<FailedRole> => {
          try {
            const response = (await http.request({
              url: `${serverUrl}/roles/${encodeURIComponent(roleId)}`,
              method: 'GET',
            } as unknown as Parameters<typeof http.request>[0])) as {data?: {name?: string}};
            return {id: roleId, name: response.data?.name ?? roleId};
          } catch {
            return {id: roleId, name: roleId};
          }
        }),
      );
      throw new AddUserToRolesError(failedRoles);
    },
  });
}
