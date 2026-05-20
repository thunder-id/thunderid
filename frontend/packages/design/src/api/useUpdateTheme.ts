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

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import DesignQueryKeys from '../constants/design-query-keys';
import type {UpdateThemeRequest} from '../models/requests';
import type {ThemeResponse} from '../models/responses';

interface UpdateThemeParams {
  themeId: string;
  data: UpdateThemeRequest;
}

/**
 * Custom hook to update an existing theme configuration in the server.
 *
 * @returns TanStack Query mutation object for updating theme configurations
 */
export default function useUpdateTheme(): UseMutationResult<ThemeResponse, Error, UpdateThemeParams> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<ThemeResponse, Error, UpdateThemeParams>({
    mutationFn: async ({themeId, data}: UpdateThemeParams): Promise<ThemeResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ThemeResponse;
      } = await http.request({
        url: `${serverUrl}/design/themes/${themeId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_, {themeId}) => {
      queryClient.invalidateQueries({queryKey: [DesignQueryKeys.THEME, themeId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [DesignQueryKeys.THEMES]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
