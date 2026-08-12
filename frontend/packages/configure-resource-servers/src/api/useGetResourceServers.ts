// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import type {ResourceServerListResponse} from '../models/resource-server';

export default function useGetResourceServers(params?: {
  limit?: number;
  offset?: number;
}): UseQueryResult<ResourceServerListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<ResourceServerListResponse>({
    queryKey: [ResourceServerQueryKeys.RESOURCE_SERVERS, {limit, offset}],
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<ResourceServerListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      const response: {data: ResourceServerListResponse} = await http.request({
        url: `${serverUrl}/resource-servers?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
