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
 */
export interface RemoveGroupMembersVariables {
  groupId: string;
  members: Member[];
}

/**
 * Custom React hook to remove members from an existing group.
 *
 * @returns TanStack Query mutation object for removing group members
 */
export default function useRemoveGroupMembers(): UseMutationResult<void, Error, RemoveGroupMembersVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<void, Error, RemoveGroupMembersVariables>({
    mutationFn: async ({groupId, members}: RemoveGroupMembersVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();

      // One member per execution, because a criterion names one principal and the flow revokes for the
      // principal that actually lost the grant.
      //
      // This is not atomic, and the native endpoint it replaces was: a refusal partway through leaves
      // the earlier members removed and revoked while the mutation reports failure. Continuing past a
      // refusal would be worse, since the caller would be told everything succeeded. Whoever surfaces
      // the error should re-read the list rather than assume nothing changed.
      let ranViaFlow = false;

      for (const member of members) {
        ranViaFlow = await removeGroupMemberViaFlow(http as unknown as HttpLike, serverUrl, groupId, member.id);
        if (!ranViaFlow) {
          break;
        }
      }
      if (ranViaFlow) {
        return;
      }
      await http.request({
        url: `${serverUrl}/groups/${groupId}/members/remove`,
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
      showToast(t('removeMember.success'), 'success');
    },
  });
}
