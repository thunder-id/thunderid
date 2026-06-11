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

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import type {Action, CreateActionRequest} from '../models/resource-server';

export default function useCreateAction(
  resourceServerId: string,
  resourceId?: string,
): UseMutationResult<Action, Error, CreateActionRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<Action, Error, CreateActionRequest>({
    mutationFn: async (data: CreateActionRequest): Promise<Action> => {
      const serverUrl = getServerUrl();
      const url = resourceId
        ? `${serverUrl}/resource-servers/${resourceServerId}/resources/${resourceId}/actions`
        : `${serverUrl}/resource-servers/${resourceServerId}/actions`;

      const response: {data: Action} = await http.request({
        url,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      if (resourceId) {
        void queryClient.invalidateQueries({
          queryKey: [ResourceServerQueryKeys.RESOURCE_ACTIONS, resourceServerId, resourceId],
        });
      } else {
        void queryClient.invalidateQueries({
          queryKey: [ResourceServerQueryKeys.SERVER_ACTIONS, resourceServerId],
        });
      }
    },
  });
}
