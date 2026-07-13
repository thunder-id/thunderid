/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionInstanceSummary, ConnectionType} from '../models/connection';

export interface UseConnectionInstancesOptions {
  enabled?: boolean;
}

/**
 * Fetch the configured instances for a connection type (GET /connections/{type}).
 */
export default function useConnectionInstances(
  type: ConnectionType,
  options?: UseConnectionInstancesOptions,
): UseQueryResult<ConnectionInstanceSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionInstanceSummary[]>({
    queryKey: [ConnectionQueryKeys.CONNECTION_INSTANCES, type],
    enabled: options?.enabled ?? true,
    queryFn: async (): Promise<ConnectionInstanceSummary[]> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionInstanceSummary[];
      } = await http.request({
        url: `${serverUrl}/connections/${type}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
