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
import DesignQueryKeys from '../constants/design-query-keys';
import type {ThemeResponse} from '../models/responses';

/**
 * Custom hook to fetch a single theme configuration by ID from the server.
 *
 * @param themeId - The unique identifier of the theme configuration
 * @returns TanStack Query result object with theme data
 */
export default function useGetTheme(themeId: string): UseQueryResult<ThemeResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ThemeResponse>({
    queryKey: [DesignQueryKeys.THEME, themeId],
    queryFn: async (): Promise<ThemeResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ThemeResponse;
      } = await http.request({
        url: `${serverUrl}/design/themes/${themeId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(themeId),
  });
}
