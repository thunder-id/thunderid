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
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Group} from '../models/group';
import type {UpdateGroupRequest} from '../models/requests';

/**
 * Variables for the update group mutation.
 */
export interface UpdateGroupVariables {
  groupId: string;
  data: UpdateGroupRequest;
}

/**
 * Custom React hook to update an existing group.
 *
 * @returns TanStack Query mutation object for updating groups
 */
export default function useUpdateGroup(): UseMutationResult<Group, Error, UpdateGroupVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<Group, Error, UpdateGroupVariables>({
    mutationFn: async ({groupId, data}: UpdateGroupVariables): Promise<Group> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: Group;
      } = await http.request({
        url: `${serverUrl}/groups/${groupId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, {groupId}) => {
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUP, groupId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUPS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUP_MEMBERS, groupId]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
    onError: () => {
      showToast(t('update.error'), 'error');
    },
  });
}
