// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';

export interface MappingTargetGroup {
  id: string;
  name: string;
}

export interface GroupsForMappingResponse {
  groups: MappingTargetGroup[];
}

export interface UseGetGroupsForMappingParams {
  limit?: number;
  offset?: number;
  /** The organization unit of the connection; only groups it can grant are listed. */
  ouId?: string;
}

/**
 * Fetches a paginated list of groups for the Authorization Mapping picker: the groups the
 * connection's organization unit owns, since groups are never shared. Nothing is fetched until
 * `ouId` is known, so groups from other organization units are never offered. Depends only on the
 * groups REST endpoint, not on @thunderid/configure-groups, since that package depends (through
 * configure-applications) back on this package, and importing it here would create a cycle.
 */
export default function useGetGroupsForMapping(
  params?: UseGetGroupsForMappingParams,
): UseQueryResult<GroupsForMappingResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, ouId} = params ?? {};

  return useQuery<GroupsForMappingResponse>({
    queryKey: [ConnectionQueryKeys.MAPPING_GROUPS, {limit, offset, ouId}],
    placeholderData: keepPreviousData,
    enabled: Boolean(ouId),
    queryFn: async (): Promise<GroupsForMappingResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
        filter: `ouId eq "${ouId}"`,
      });

      const response: {data: GroupsForMappingResponse} = await http.request({
        url: `${serverUrl}/groups?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
