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
import type {ConnectionListResponse, ConnectionTypeSummary} from '../models/connection';

/**
 * Fetch the available connection types with their configured status and instance count
 * (GET /connections). Unwraps the `connections` array from the response envelope.
 */
export default function useConnections(): UseQueryResult<ConnectionTypeSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionTypeSummary[]>({
    queryKey: [ConnectionQueryKeys.CONNECTION_TYPES],
    queryFn: async (): Promise<ConnectionTypeSummary[]> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionListResponse;
      } = await http.request({
        url: `${serverUrl}/connections`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.connections;
    },
  });
}
