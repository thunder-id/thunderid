// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {UpdateRoleRequest} from '../models/requests';
import type {Role} from '../models/role';
import {updateRoleNatively, updateRoleViaFlow, type HttpLike} from '../utils/roleAdministrationFlow';

export const ROLE_MUTATION_KEY = ['update-role'] as const;

export interface UpdateRoleVariables {
  roleId: string;
  data: UpdateRoleRequest;
  /** The role as the caller last read it, which decides whether the edit removes a permission. */
  current: Role;
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
  const mode = useAdministrationOperation<[UpdateRoleVariables], Role>('roles', 'update');

  return useMutation<Role, Error, UpdateRoleVariables>({
    mutationKey: ROLE_MUTATION_KEY,
    mutationFn: async (variables: UpdateRoleVariables): Promise<Role> => {
      const client = http as unknown as HttpLike;
      const {roleId, data, current} = variables;

      if (typeof mode === 'function') {
        return mode(variables);
      }
      if (mode === AdministrationModes.NATIVE) {
        return updateRoleNatively(client, getServerUrl(), roleId, data);
      }

      return updateRoleViaFlow(client, getServerUrl(), roleId, data, current);
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
