// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import type {ResourceServerListResponse} from '../models/resource-server';

/**
 * The resource servers, a page at a time. With a `projectId`, only that project's resource servers are
 * listed; without one, the control plane leaves out the resource servers of a project unless `includeAll` is set.
 */
export default function useGetResourceServers(params?: {
  limit?: number;
  offset?: number;
  projectId?: string;
  /** Lists only the resource servers this organization unit owns. */
  ouId?: string;
  /** Without a project, also lists the resource servers of a project, which the control plane otherwise leaves out. */
  includeAll?: boolean;
  enabled?: boolean;
}): UseQueryResult<ResourceServerListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, projectId, ouId, includeAll, enabled = true} = params ?? {};

  return useQuery<ResourceServerListResponse>({
    queryKey: [ResourceServerQueryKeys.RESOURCE_SERVERS, {limit, offset, projectId, ouId, includeAll}],
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
      if (includeAll) {
        queryParams.append('includeAll', 'true');
      }

      const response: {data: ResourceServerListResponse} = await http.request({
        url: `${serverUrl}/resource-servers?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
