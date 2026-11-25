// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import UserQueryKeys from '../constants/user-query-keys';

/**
 * Custom hook to delete a user by ID.
 *
 * The deletion calls `DELETE /users/{id}`; the server revokes the user's tokens and ends their
 * sessions before the record is removed. A console that declares its own operation runs that instead.
 *
 * @returns TanStack Query mutation object for deleting users
 */
export default function useDeleteUser(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('users');
  const {showToast} = useToast();
  const mode = useAdministrationOperation<[string], void>('users', 'delete');

  return useMutation<void, Error, string>({
    mutationFn: async (userId: string): Promise<void> => {
      if (typeof mode === 'function') {
        await mode(userId);
        return;
      }
      await http.request({
        url: `${getServerUrl()}/users/${userId}`,
        method: 'DELETE',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, userId) => {
      queryClient.removeQueries({queryKey: [UserQueryKeys.USER, userId]});
      queryClient.invalidateQueries({queryKey: [UserQueryKeys.USERS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success'), 'success');
    },
  });
}
