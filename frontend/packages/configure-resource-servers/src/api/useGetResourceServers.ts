// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import type {ResourceServerListResponse} from '../models/resource-server';

/**
 * The resource servers, a page at a time. With a `projectId`, only that project's resource servers are
 * listed; without one, every resource server is.
 */
export default function useGetResourceServers(params?: {
  limit?: number;
  offset?: number;
  projectId?: string;
  /** Lists only the resource servers this organization unit owns. */
  ouId?: string;
  enabled?: boolean;
}): UseQueryResult<ResourceServerListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, projectId, ouId, enabled = true} = params ?? {};

  return useQuery<ResourceServerListResponse>({
    queryKey: [ResourceServerQueryKeys.RESOURCE_SERVERS, {limit, offset, projectId, ouId}],
    placeholderData: keepPreviousData,
    enabled,
    queryFn: async (): Promise<ResourceServerListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });
      if (projectId) {
        queryParams.append('projectId', projectId);
      }
      if (ouId) {
        queryParams.append('filter', `ouId eq "${ouId}"`);
      }

      const response: {data: ResourceServerListResponse} = await http.request({
        url: `${serverUrl}/resource-servers?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
