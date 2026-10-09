// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Member} from '../models/group';
import {removeGroupMemberViaFlow} from '../utils/groupAdministrationFlow';

/**
 * Variables for the remove group members mutation.
 *
 * One member per call: a loop of flow executions could not be rolled back if one failed.
 */
export interface RemoveGroupMembersVariables {
  groupId: string;
  member: Member;
}

/**
 * Custom React hook to remove a member from an existing group.
 *
 * @returns TanStack Query mutation object for removing a group member
 */
export default function useRemoveGroupMembers(): UseMutationResult<void, Error, RemoveGroupMembersVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<void, Error, RemoveGroupMembersVariables>({
    mutationFn: async ({groupId, member}: RemoveGroupMembersVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();

      if (await removeGroupMemberViaFlow(http as unknown as HttpLike, serverUrl, groupId, member.id)) {
        return;
      }
      await http.request({
        url: `${serverUrl}/groups/${groupId}/members/remove`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: {members: [member]},
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
      showToast(t('removeMember.success'), 'success');
    },
  });
}
