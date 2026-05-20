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
import {useConfig, useToast} from '@thunderid/contexts';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Member} from '../models/group';

/**
 * Variables for the add group members mutation.
 */
export interface AddGroupMembersVariables {
  groupId: string;
  members: Member[];
}

/**
 * Custom React hook to add members to an existing group.
 *
 * @returns TanStack Query mutation object for adding group members
 */
export default function useAddGroupMembers(): UseMutationResult<void, Error, AddGroupMembersVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<void, Error, AddGroupMembersVariables>({
    mutationFn: async ({groupId, members}: AddGroupMembersVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/groups/${groupId}/members/add`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: {members},
      } as unknown as Parameters<typeof http.request>[0]);
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
      showToast(t('addMember.success'), 'success');
    },
    onError: () => {
      showToast(t('addMember.error'), 'error');
    },
  });
}
