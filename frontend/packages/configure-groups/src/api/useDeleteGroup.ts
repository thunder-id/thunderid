// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import {deleteGroupViaFlow} from '../utils/groupAdministrationFlow';

/**
 * Custom React hook to delete a group.
 *
 * @returns TanStack Query mutation object for deleting groups
 */
export default function useDeleteGroup(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('groups');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (groupId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();

      // Deleting a group takes away every scope its roles conveyed, for every member. The flow revokes
      // those tokens first; the native endpoint below revokes nothing, so it is only reached when no
      // flow is configured.
      if (await deleteGroupViaFlow(http as unknown as HttpLike, serverUrl, groupId)) {
        return;
      }
      await http.request({
        url: `${serverUrl}/groups/${groupId}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, groupId) => {
      queryClient.removeQueries({queryKey: [GroupQueryKeys.GROUP, groupId]});
      queryClient.invalidateQueries({queryKey: [GroupQueryKeys.GROUPS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
  });
}
