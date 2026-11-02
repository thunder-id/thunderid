// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {getRole, RoleQueryKeys, useManagementRequestConfig, type Role} from '@thunderid/react';

/**
 * Fetches a role, for the heading of its details page.
 *
 * @param roleId - The ID of the role to fetch.
 * @returns TanStack Query result holding the role.
 */
export default function useGetRole(roleId: string): UseQueryResult<Role> {
  const config = useManagementRequestConfig('roles');

  return useQuery<Role>({
    enabled: Boolean(roleId),
    queryFn: (): Promise<Role> => getRole({...config, roleId}),
    queryKey: [RoleQueryKeys.ROLE, roleId],
  });
}
