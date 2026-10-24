// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GroupQueryKeys from '../constants/group-query-keys';
import {deleteGroupNatively, deleteGroupViaFlow, type HttpLike} from '../utils/groupAdministrationFlow';

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
  const mode = useAdministrationOperation<[string], void>('groups', 'delete');

  return useMutation<void, Error, string>({
    mutationFn: async (groupId: string): Promise<void> => {
      const client = http as unknown as HttpLike;

      if (typeof mode === 'function') {
        await mode(groupId);
        return;
      }
      if (mode === AdministrationModes.NATIVE) {
        await deleteGroupNatively(client, getServerUrl(), groupId);
        return;
      }
      await deleteGroupViaFlow(client, getServerUrl(), groupId);
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
