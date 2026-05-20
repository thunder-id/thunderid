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

import {useThunderID} from '@thunderid/react';
import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import DesignQueryKeys from '../constants/design-query-keys';
import type {LayoutResponse} from '../models/responses';

/**
 * Custom hook to fetch a single layout configuration by ID from the server.
 *
 * @param layoutId - The unique identifier of the layout configuration
 * @returns TanStack Query result object with layout data
 */
export default function useGetLayout(layoutId: string): UseQueryResult<LayoutResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<LayoutResponse>({
    queryKey: [DesignQueryKeys.LAYOUT, layoutId],
    queryFn: async (): Promise<LayoutResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: LayoutResponse;
      } = await http.request({
        url: `${serverUrl}/design/layouts/${layoutId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(layoutId),
  });
}
