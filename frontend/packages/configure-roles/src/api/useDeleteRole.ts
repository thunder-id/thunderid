// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import {deleteRoleNatively, deleteRoleViaFlow, type HttpLike} from '../utils/roleAdministrationFlow';

/**
 * Custom React hook to delete a role.
 *
 * @returns TanStack Query mutation object for deleting roles
 */
export default function useDeleteRole(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('roles');
  const {showToast} = useToast();
  const mode = useAdministrationOperation<[string], void>('roles', 'delete');

  return useMutation<void, Error, string>({
    mutationFn: async (roleId: string): Promise<void> => {
      const client = http as unknown as HttpLike;

      if (typeof mode === 'function') {
        await mode(roleId);
        return;
      }
      if (mode === AdministrationModes.NATIVE) {
        await deleteRoleNatively(client, getServerUrl(), roleId);
        return;
      }
      await deleteRoleViaFlow(client, getServerUrl(), roleId);
    },
    onSuccess: (_data, roleId) => {
      queryClient.removeQueries({queryKey: [RoleQueryKeys.ROLE, roleId]});
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLES]}).catch(() => {
        /* noop */
      });
      showToast(t('delete.success'), 'success');
    },
  });
}
