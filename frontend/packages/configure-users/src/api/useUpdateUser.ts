/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import type {User} from '@thunderid/types';
import {useTranslation} from 'react-i18next';
import UserQueryKeys from '../constants/user-query-keys';
import type {UpdateUserRequest} from '../models/users';

/**
 * Variables for the update user mutation.
 */
export interface UpdateUserVariables {
  userId: string;
  data: UpdateUserRequest;
}

/**
 * Custom hook to update an existing user.
 *
 * @returns TanStack Query mutation object for updating users
 */
export default function useUpdateUser(): UseMutationResult<User, Error, UpdateUserVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('users');
  const {showToast} = useToast();

  return useMutation<User, Error, UpdateUserVariables>({
    mutationFn: async ({userId, data}: UpdateUserVariables): Promise<User> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: User;
      } = await http.request({
        url: `${serverUrl}/users/${userId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USER, variables.userId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USERS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
    onError: () => {
      showToast(t('update.error'), 'error');
    },
  });
}
