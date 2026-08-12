// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GroupQueryKeys from '../constants/group-query-keys';
import type {GroupListResponse} from '../models/group';
import type {GroupListParams} from '../models/requests';

/**
 * Custom React hook to fetch a paginated list of groups.
 *
 * @param params - Optional pagination parameters
 * @returns TanStack Query result object containing groups list data
 */
export default function useGetGroups(params?: GroupListParams): UseQueryResult<GroupListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<GroupListResponse>({
    queryKey: [GroupQueryKeys.GROUPS, {limit, offset}],
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<GroupListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });

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
