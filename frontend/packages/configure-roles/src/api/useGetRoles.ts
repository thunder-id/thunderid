// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import RoleQueryKeys from '../constants/role-query-keys';
import type {RoleListParams} from '../models/requests';
import type {RoleListResponse} from '../models/role';

/**
 * Custom React hook to fetch a paginated list of roles.
 *
 * @param params - Optional pagination parameters
 * @returns TanStack Query result object containing roles list data
 */
export default function useGetRoles(params?: RoleListParams): UseQueryResult<RoleListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<RoleListResponse>({
    queryKey: [RoleQueryKeys.ROLES, {limit, offset}],
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<RoleListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });

      const response: {data: RoleListResponse} = await http.request({
        url: `${serverUrl}/roles?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
