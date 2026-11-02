// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Member} from '../models/group';
import {removeGroupMemberNatively, removeGroupMemberViaFlow, type HttpLike} from '../utils/groupAdministrationFlow';

/**
 * Variables for the remove group member mutation.
 *
 * One member per call: a loop of flow executions could not be rolled back if one failed.
 */
export interface RemoveGroupMemberVariables {
  groupId: string;
  member: Member;
}

/**
 * Custom React hook to remove a member from an existing group.
 *
 * @returns TanStack Query mutation object for removing a group member
 */
export default function useRemoveGroupMember(): UseMutationResult<void, Error, RemoveGroupMemberVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();
  const mode = useAdministrationOperation<[RemoveGroupMemberVariables], void>('groups', 'removeMember');

  return useMutation<void, Error, RemoveGroupMemberVariables>({
    mutationFn: async (variables: RemoveGroupMemberVariables): Promise<void> => {
      const client = http as unknown as HttpLike;
      const {groupId, member} = variables;

      if (typeof mode === 'function') {
        await mode(variables);
        return;
      }
      if (mode === AdministrationModes.NATIVE) {
        await removeGroupMemberNatively(client, getServerUrl(), groupId, member);
        return;
      }
      await removeGroupMemberViaFlow(client, getServerUrl(), groupId, member);
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
