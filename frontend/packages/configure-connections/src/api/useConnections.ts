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
import type {ConnectionInstanceCategory, ConnectionListResponse} from '../models/connection';

export interface UseConnectionsParams {
  category?: ConnectionInstanceCategory;
  limit?: number;
  offset?: number;
}

export interface UseConnectionsOptions {
  enabled?: boolean;
}

/**
 * Fetch a paginated list of the configured connection instances (GET /connections),
 * optionally filtered by functional category. Returns the full paginated envelope.
 */
export default function useConnections(
  params?: UseConnectionsParams,
  options?: UseConnectionsOptions,
): UseQueryResult<ConnectionListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {category, limit, offset} = params ?? {};

  return useQuery<ConnectionListResponse>({
    queryKey: [ConnectionQueryKeys.CONNECTIONS, {category, limit, offset}],
    enabled: options?.enabled ?? true,
    queryFn: async (): Promise<ConnectionListResponse> => {
      const serverUrl: string = getServerUrl();
      const searchParams: URLSearchParams = new URLSearchParams();
      if (category) {
        searchParams.set('category', category);
      }
      if (limit !== undefined) {
        searchParams.set('limit', String(limit));
      }
      if (offset !== undefined) {
        searchParams.set('offset', String(offset));
      }
      const queryString: string = searchParams.toString();
      const query: string = queryString ? `?${queryString}` : '';
      const response: {
        data: ConnectionListResponse;
      } = await http.request({
        url: `${serverUrl}/connections${query}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
