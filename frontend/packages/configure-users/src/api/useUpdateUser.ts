// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {UserQueryKeys as SdkUserQueryKeys, useThunderID} from '@thunderid/react';
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
  const {http, invalidator} = useThunderID();
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
      // Lists the SDK's admin components render keep their own state, so they refresh on this too.
      invalidator?.invalidate([SdkUserQueryKeys.USERS]);
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USER, variables.userId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USERS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
  });
}
