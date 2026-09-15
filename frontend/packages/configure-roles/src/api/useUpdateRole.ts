// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {HttpLike} from '@thunderid/utils';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {UpdateRoleRequest} from '../models/requests';
import type {Role} from '../models/role';
import {permissionsRemoved, updateRolePermissionsViaFlow} from '../utils/roleAdministrationFlow';

export const ROLE_MUTATION_KEY = ['update-role'] as const;

export interface UpdateRoleVariables {
  roleId: string;
  data: UpdateRoleRequest;
}

/**
 * Custom React hook to update an existing role.
 *
 * @returns TanStack Query mutation object for updating roles
 */
export default function useUpdateRole(): UseMutationResult<Role, Error, UpdateRoleVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('roles');
  const {showToast} = useToast();

  return useMutation<Role, Error, UpdateRoleVariables>({
    mutationKey: ROLE_MUTATION_KEY,
    mutationFn: async ({roleId, data}: UpdateRoleVariables): Promise<Role> => {
      const serverUrl: string = getServerUrl();

      // Only a permission removal needs revoking, so the current set is read to find out whether this
      // edit takes anything away. A rename or an added permission takes no scope from anyone, and the
      // flow revokes the role's whole set rather than a delta, so running it for those would revoke
      // every holder's tokens for nothing.
      const existing: {data?: Role} = await http.request({
        url: `${serverUrl}/roles/${roleId}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      if (permissionsRemoved(existing?.data?.permissions, data.permissions)) {
        // The flow revokes the lost scopes and writes the new permission set, but leaves the role's
        // other attributes alone. The update below therefore still runs, to apply the name, description
        // and organization unit this edit may also carry.
        //
        // The two writes are not atomic. If the update below fails, the permissions have already been
        // replaced and the lost scopes revoked, so the edit is half applied. That is the safe half to
        // land first: the revocation matches the permissions that were written, and re-submitting the
        // edit is idempotent.
        await updateRolePermissionsViaFlow(http as unknown as HttpLike, serverUrl, roleId, data.permissions);
      }
      const response: {data: Role} = await http.request({
        url: `${serverUrl}/roles/${roleId}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data, {roleId}) => {
      queryClient.setQueryData<Role>([RoleQueryKeys.ROLE, roleId], (old) =>
        old ? {...old, name: data.name, description: data.description, permissions: data.permissions ?? []} : data,
      );
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLE, roleId]}).catch(() => {
        /* noop */
      });
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLES]}).catch(() => {
        /* noop */
      });
      showToast(t('update.success'), 'success');
    },
  });
}
