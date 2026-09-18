// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import {deleteRoleViaFlow} from '../utils/roleAdministrationFlow';

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

  return useMutation<void, Error, string>({
    mutationFn: async (roleId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();

      // Deleting a role takes its scopes from everyone holding it. The flow revokes those tokens
      // before the role goes; the native endpoint below revokes nothing, which is why it is only
      // reached when no flow is configured.
      if (await deleteRoleViaFlow(http as unknown as HttpLike, serverUrl, roleId)) {
        return;
      }
      await http.request({
        url: `${serverUrl}/roles/${roleId}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
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
