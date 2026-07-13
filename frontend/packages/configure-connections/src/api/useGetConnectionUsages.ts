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
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionType, ConnectionUsagesResponse} from '../models/connection';

/**
 * Fetch the resources that reference a connection instance, such as flows that use it
 * (GET /connections/{type}/{id}/usages). Used to populate the pre-delete confirmation dialog.
 *
 * @param type - The connection type
 * @param id - The connection instance identifier
 * @param enabled - Whether the query should run (default true)
 */
export default function useGetConnectionUsages(
  type: ConnectionType,
  id: string | undefined,
  enabled = true,
): UseQueryResult<ConnectionUsagesResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionUsagesResponse>({
    queryKey: [ConnectionQueryKeys.CONNECTION_USAGES, type, id],
    enabled: Boolean(id) && enabled,
    queryFn: async (): Promise<ConnectionUsagesResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionUsagesResponse;
      } = await http.request({
        url: `${serverUrl}/connections/${type}/${id}/usages`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
