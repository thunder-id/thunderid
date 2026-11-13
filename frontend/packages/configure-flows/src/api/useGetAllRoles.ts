// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig, useProjectScope} from '@thunderid/contexts';
import {RoleQueryKeys, useThunderID, type RoleListResponse, type RoleSummary} from '@thunderid/react';
import fetchAllPages from '../utils/fetchAllPages';

/**
 * Fetches every role, paging through the list, so a picker can offer them all by name. In a project, only that
 * project's roles are fetched.
 *
 * @returns TanStack Query result holding all roles.
 */
export default function useGetAllRoles(): UseQueryResult<RoleSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {projectId} = useProjectScope();

  return useQuery<RoleSummary[]>({
    queryKey: [RoleQueryKeys.ROLES, 'all', {projectId}],
    queryFn: (): Promise<RoleSummary[]> =>
      fetchAllPages<RoleSummary>(async (limit, offset) => {
        const queryParams = new URLSearchParams({
          limit: limit.toString(),
          offset: offset.toString(),
          include: 'display',
        });
        if (projectId) {
          queryParams.append('projectId', projectId);
        }

        const response: {data: RoleListResponse} = await http.request({
          url: `${getServerUrl()}/roles?${queryParams.toString()}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        return {items: response.data.roles, totalResults: response.data.totalResults};
      }),
  });
}
