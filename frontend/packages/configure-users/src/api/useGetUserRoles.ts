// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import UserQueryKeys from '../constants/user-query-keys';
import type {UserRole} from '../models/users';

/**
 * The most assignments one page of a role's assignments returns.
 */
const ASSIGNMENTS_PAGE_SIZE = 100;

interface RoleUserAssignment {
  id: string;
  ouId?: string;
  isReadOnly?: boolean;
}

interface RoleUserAssignmentListResponse {
  totalResults: number;
  assignments: RoleUserAssignment[];
}

/**
 * Custom hook to find which of the given roles are assigned to a user itself. Roles held through a group
 * are not found.
 *
 * No API lists a user's roles, so it reads each role's user assignments, a page at a time, until it finds
 * the user or runs out. A role a user share mandates comes back read only.
 *
 * @param userId - The unique identifier of the user
 * @param roles - The roles to look for the user in; nothing is read until they are given
 * @returns TanStack Query result with the roles assigned to the user, in the order given
 */
export default function useGetUserRoles(userId: string, roles: UserRole[] | undefined): UseQueryResult<UserRole[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const knownRoles: UserRole[] = roles ?? [];
  const roleIds: string[] = knownRoles.map((role) => role.id);

  return useQuery<UserRole[]>({
    queryKey: [UserQueryKeys.USER_ROLES, userId, roleIds],
    queryFn: async (): Promise<UserRole[]> => {
      const serverUrl: string = getServerUrl();

      const findAssignment = async (roleId: string): Promise<RoleUserAssignment | undefined> => {
        let offset = 0;
        for (;;) {
          const queryParams = new URLSearchParams({
            type: 'user',
            limit: String(ASSIGNMENTS_PAGE_SIZE),
            offset: String(offset),
          });
          const response: {data: RoleUserAssignmentListResponse} = await http.request({
            url: `${serverUrl}/roles/${encodeURIComponent(roleId)}/assignments?${queryParams.toString()}`,
            method: 'GET',
            headers: {
              'Content-Type': 'application/json',
            },
          } as unknown as Parameters<typeof http.request>[0]);

          const {assignments, totalResults} = response.data;
          const found = assignments.find((assignment) => assignment.id === userId);
          offset += ASSIGNMENTS_PAGE_SIZE;
          if (found || assignments.length === 0 || offset >= totalResults) {
            return found;
          }
        }
      };

      const assignments = await Promise.all(knownRoles.map((role) => findAssignment(role.id)));

      return knownRoles.flatMap((role, index) => {
        const assignment = assignments[index];
        return assignment
          ? [{...role, isReadOnly: assignment.isReadOnly === true, assignmentOuId: assignment.ouId}]
          : [];
      });
    },
    enabled: Boolean(userId) && roles !== undefined,
  });
}
