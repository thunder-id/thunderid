// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import UserQueryKeys from '../constants/user-query-keys';
import type {UserRole, UserRoleListResponse} from '../models/users';

/**
 * The most roles the list returns in one page.
 */
const MAX_ASSIGNABLE_ROLES = 100;

/**
 * Custom hook to fetch the roles a user can be given. With a `projectId`, only that project's roles are
 * listed; without one, every role is.
 *
 * It reads `/roles` itself: `configure-roles` depends on this package, so its `useGetRoles` cannot be
 * used here.
 *
 * @param projectId - The project whose roles are listed
 * @param enabled - Whether the roles are read
 * @returns TanStack Query result with the roles
 */
export default function useGetAssignableRoles(
  projectId: string | undefined,
  enabled = true,
): UseQueryResult<UserRoleListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<UserRoleListResponse>({
    queryKey: [UserQueryKeys.ASSIGNABLE_ROLES, {projectId}],
    queryFn: async (): Promise<UserRoleListResponse> => {
      const serverUrl: string = getServerUrl();
      const roles: UserRole[] = [];
      let offset = 0;
      // The server caps a page, so the pages are read until the roles run out.
      for (;;) {
        const queryParams = new URLSearchParams({limit: String(MAX_ASSIGNABLE_ROLES), offset: String(offset)});
        if (projectId) {
          queryParams.append('projectId', projectId);
        }

        const response: {data: UserRoleListResponse} = await http.request({
          url: `${serverUrl}/roles?${queryParams.toString()}`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        roles.push(...response.data.roles);
        offset += MAX_ASSIGNABLE_ROLES;
        if (response.data.roles.length === 0 || offset >= response.data.totalResults) {
          return {totalResults: response.data.totalResults, count: roles.length, roles};
        }
      }
    },
    enabled,
  });
}
