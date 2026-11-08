// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GroupQueryKeys from '../constants/group-query-keys';
import type {GroupListResponse} from '../models/group';
import type {GroupListParams} from '../models/requests';

/**
 * Custom React hook to fetch a paginated list of groups. With a `projectId`, only that project's groups are
 * listed; without one, every group is.
 *
 * @param params - Optional pagination parameters and project
 * @returns TanStack Query result object containing groups list data
 */
export default function useGetGroups(params?: GroupListParams): UseQueryResult<GroupListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, filter, projectId} = params ?? {};

  return useQuery<GroupListResponse>({
    queryKey: [GroupQueryKeys.GROUPS, {limit, offset, filter, projectId}],
    // Keep the previous page on screen while paging, but never show one filter's or project's groups as
    // another's.
    placeholderData: (previousData, previousQuery) => {
      const previous = previousQuery?.queryKey[1] as {filter?: string; projectId?: string} | undefined;
      return previous?.filter === filter && previous?.projectId === projectId ? previousData : undefined;
    },
    queryFn: async (): Promise<GroupListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });
      if (filter) {
        queryParams.append('filter', filter);
      }
      if (projectId) {
        queryParams.append('projectId', projectId);
      }

      const response: {
        data: GroupListResponse;
      } = await http.request({
        url: `${serverUrl}/groups?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
