// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {RoleQueryKeys as SdkRoleQueryKeys, useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {CreateRoleRequest} from '../models/requests';
import type {Role} from '../models/role';

/**
 * Custom React hook to create a new role.
 *
 * @returns TanStack Query mutation object for creating roles
 */
export default function useCreateRole(): UseMutationResult<Role, Error, CreateRoleRequest> {
  const {http, invalidator} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('roles');
  const {showToast} = useToast();

  return useMutation<Role, Error, CreateRoleRequest>({
    mutationFn: async (roleData: CreateRoleRequest): Promise<Role> => {
      const serverUrl: string = getServerUrl();
      const response: {data: Role} = await http.request({
        url: `${serverUrl}/roles`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: roleData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      // Lists the SDK's admin components render keep their own state, so they refresh on this too.
      invalidator?.invalidate([SdkRoleQueryKeys.ROLES]);
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLES]}).catch(() => {
        /* noop */
      });
      showToast(t('create.success'), 'success');
    },
  });
}
