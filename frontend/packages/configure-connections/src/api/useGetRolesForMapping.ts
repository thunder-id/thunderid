// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';

export interface MappingTargetRole {
  id: string;
  name: string;
}

export interface RolesForMappingResponse {
  roles: MappingTargetRole[];
}

export interface UseGetRolesForMappingParams {
  limit?: number;
  offset?: number;
  /** The organization unit of the connection; only roles it can grant are listed. */
  ouId?: string;
}

/**
 * Fetches a paginated list of roles for the Authorization Mapping picker: the roles the connection's
 * organization unit owns or has shared with it. Nothing is fetched until `ouId` is known, so roles
 * from other organization units are never offered. Depends only on the roles REST endpoint, not on
 * @thunderid/configure-roles, since that package depends (through configure-applications) back on
 * this package, and importing it here would create a cycle.
 */
export default function useGetRolesForMapping(
  params?: UseGetRolesForMappingParams,
): UseQueryResult<RolesForMappingResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, ouId} = params ?? {};

  return useQuery<RolesForMappingResponse>({
    queryKey: [ConnectionQueryKeys.MAPPING_ROLES, {limit, offset, ouId}],
    placeholderData: keepPreviousData,
    enabled: Boolean(ouId),
    queryFn: async (): Promise<RolesForMappingResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
        filter: `ouId eq "${ouId}"`,
      });

      const response: {data: RolesForMappingResponse} = await http.request({
        url: `${serverUrl}/roles?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
