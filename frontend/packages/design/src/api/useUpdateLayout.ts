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
import type {UpdateLayoutRequest} from '../models/requests';
import type {LayoutResponse} from '../models/responses';

interface UpdateLayoutParams {
  layoutId: string;
  data: UpdateLayoutRequest;
}

/**
 * Custom hook to update an existing layout configuration in the server.
 *
 * @returns TanStack Query mutation object for updating layout configurations
 */
export default function useUpdateLayout(): UseMutationResult<LayoutResponse, Error, UpdateLayoutParams> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<LayoutResponse, Error, UpdateLayoutParams>({
    mutationFn: async ({layoutId, data}: UpdateLayoutParams): Promise<LayoutResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: LayoutResponse;
      } = await http.request({
        url: `${serverUrl}/design/layouts/${layoutId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_, {layoutId}) => {
      queryClient.invalidateQueries({queryKey: [DesignQueryKeys.LAYOUT, layoutId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [DesignQueryKeys.LAYOUTS]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
