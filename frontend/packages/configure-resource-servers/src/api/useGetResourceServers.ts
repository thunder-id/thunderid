/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
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
