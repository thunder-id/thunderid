// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {UserQueryKeys as SdkUserQueryKeys, useThunderID} from '@thunderid/react';
import type {User} from '@thunderid/types';
import {useTranslation} from 'react-i18next';
import UserQueryKeys from '../constants/user-query-keys';
import type {CreateUserRequest} from '../models/users';

/**
 * Custom hook to create a new user.
 *
 * @returns TanStack Query mutation object for creating users
 */
export default function useCreateUser(): UseMutationResult<User, Error, CreateUserRequest> {
  const {http, invalidator} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('users');
  const {showToast} = useToast();

  return useMutation<User, Error, CreateUserRequest>({
    mutationFn: async (userData: CreateUserRequest): Promise<User> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: User;
      } = await http.request({
        url: `${serverUrl}/users`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: userData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      // Lists the SDK's admin components render keep their own state, so they refresh on this too.
      invalidator?.invalidate([SdkUserQueryKeys.USERS]);
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USERS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('create.success'), 'success');
    },
  });
}
