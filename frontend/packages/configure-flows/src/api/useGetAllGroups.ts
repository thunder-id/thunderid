// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig, useProjectScope} from '@thunderid/contexts';
import {GroupQueryKeys, useThunderID, type Group, type GroupListResponse} from '@thunderid/react';
import fetchAllPages from '../utils/fetchAllPages';

/**
 * Fetches every group, paging through the list, so a picker can offer them all by name. In a project, only that
 * project's groups are fetched.
 *
 * @returns TanStack Query result holding all groups.
 */
export default function useGetAllGroups(): UseQueryResult<Group[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {projectId} = useProjectScope();

  return useQuery<Group[]>({
    queryKey: [GroupQueryKeys.GROUPS, 'all', {projectId}],
    queryFn: (): Promise<Group[]> =>
      fetchAllPages<Group>(async (limit, offset) => {
        const queryParams = new URLSearchParams({
          limit: limit.toString(),
          offset: offset.toString(),
          include: 'display',
        });
        if (projectId) {
          queryParams.append('projectId', projectId);
        }

        const response: {data: GroupListResponse} = await http.request({
          url: `${getServerUrl()}/groups?${queryParams.toString()}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        return {items: response.data.groups, totalResults: response.data.totalResults};
      }),
  });
}
