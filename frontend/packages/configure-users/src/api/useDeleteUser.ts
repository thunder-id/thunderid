// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useAdministrationActions, useConfig, useToast, type AdministrationHttpLike} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import UserQueryKeys from '../constants/user-query-keys';
import deleteUserNatively from '../utils/deleteUserNatively';

/**
 * Custom hook to delete a user by ID.
 *
 * How the deletion is carried out is supplied by the console rather than chosen here. A Data Plane
 * console installs an action that runs the configured administration flow, which revokes the user's
 * grants and terminates their sessions before the record is removed. A Control Plane console
 * installs none, and the deletion is `DELETE /users/{id}`: that plane serves no runtime, so there
 * are no grants or sessions to end first.
 *
 * @returns TanStack Query mutation object for deleting users
 */
export default function useDeleteUser(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('users');
  const {showToast} = useToast();
  const {deleteUser} = useAdministrationActions();

  return useMutation<void, Error, string>({
    mutationFn: async (userId: string): Promise<void> => {
      const client = http as unknown as AdministrationHttpLike;
      const serverUrl: string = getServerUrl();

      if (deleteUser) {
        await deleteUser(client, serverUrl, userId);

        return;
      }

      await deleteUserNatively(client, serverUrl, userId);
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
