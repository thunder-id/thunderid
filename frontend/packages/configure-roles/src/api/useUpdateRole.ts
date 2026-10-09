// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AdministrationFlowConfigKey, resolveAdministrationFlowHandle, type HttpLike} from '@thunderid/utils';
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

      // Read the current set: only a permission removal needs the revoking flow.
      const existing: {data?: Role} = await http.request({
        url: `${serverUrl}/roles/${roleId}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);
      const current: Role | undefined = existing?.data;
      const putRole = async (body: UpdateRoleRequest): Promise<Role> => {
        const response: {data: Role} = await http.request({
          url: `${serverUrl}/roles/${roleId}`,
          method: 'PUT',
          headers: {'Content-Type': 'application/json'},
          data: body,
        } as unknown as Parameters<typeof http.request>[0]);

        return response.data;
      };

      if (
        !current ||
        !permissionsRemoved(current.permissions, data.permissions) ||
        !(await resolveAdministrationFlowHandle(
          http as unknown as HttpLike,
          serverUrl,
          AdministrationFlowConfigKey.ROLE_PERMISSION_REMOVAL,
        ))
      ) {
        return putRole(data);
      }

      // The flow writes only permissions and the writes are not atomic, so other fields go first and a
      // rejected rename fails before anything is revoked.
      const otherFieldsChanged: boolean =
        data.name !== current.name ||
        (data.description ?? '') !== (current.description ?? '') ||
        data.ouId !== current.ouId;
      const updated: Role = otherFieldsChanged
        ? await putRole({...data, permissions: current.permissions ?? []})
        : current;

      if (!(await updateRolePermissionsViaFlow(http as unknown as HttpLike, serverUrl, roleId, data.permissions))) {
        // Unconfigured since the check above.
        return putRole(data);
      }

      return {...updated, permissions: data.permissions};
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
