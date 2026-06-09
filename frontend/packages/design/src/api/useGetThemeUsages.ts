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
import type {ThemeUsagesResponse} from '../models/responses';

/**
 * Custom hook to fetch resources that reference a theme.
 * Used to populate the pre-delete confirmation dialog.
 *
 * @param themeId - The unique identifier of the theme
 * @param enabled - Whether the query should run (default true)
 * @returns TanStack Query result with theme usages data
 */
export default function useGetThemeUsages(themeId: string | null, enabled = true): UseQueryResult<ThemeUsagesResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ThemeUsagesResponse>({
    queryKey: [DesignQueryKeys.THEME_USAGES, themeId],
    queryFn: async (): Promise<ThemeUsagesResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {data: ThemeUsagesResponse} = await http.request({
        url: `${serverUrl}/design/themes/${encodeURIComponent(themeId!)}/usages`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(themeId) && enabled,
  });
}
