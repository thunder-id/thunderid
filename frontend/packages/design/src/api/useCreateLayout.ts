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
import type {CreateLayoutRequest} from '../models/requests';
import type {LayoutResponse} from '../models/responses';

/**
 * Custom hook to create a new layout configuration in the server.
 *
 * @returns TanStack Query mutation object for creating layout configurations
 */
export default function useCreateLayout(): UseMutationResult<LayoutResponse, Error, CreateLayoutRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<LayoutResponse, Error, CreateLayoutRequest>({
    mutationFn: async (layoutData: CreateLayoutRequest): Promise<LayoutResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: LayoutResponse;
      } = await http.request({
        url: `${serverUrl}/design/layouts`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: layoutData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [DesignQueryKeys.LAYOUTS]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
