/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import RoleQueryKeys from '../constants/role-query-keys';
import type {UpdateRoleRequest} from '../models/requests';
import type {Role} from '../models/role';

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
    mutationFn: async ({roleId, data}: UpdateRoleVariables): Promise<Role> => {
      const serverUrl: string = getServerUrl();
      const response: {data: Role} = await http.request({
        url: `${serverUrl}/roles/${roleId}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, {roleId}) => {
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLE, roleId]}).catch(() => {
        /* noop */
      });
      queryClient.invalidateQueries({queryKey: [RoleQueryKeys.ROLES]}).catch(() => {
        /* noop */
      });
      showToast(t('update.success'), 'success');
    },
    onError: () => {
      showToast(t('update.error'), 'error');
    },
  });
}
