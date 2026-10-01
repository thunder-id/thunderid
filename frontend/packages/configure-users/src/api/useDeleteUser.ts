// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {AdministrationModes, useAdministrationOperation, useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import UserQueryKeys from '../constants/user-query-keys';
import deleteUser, {deleteUserNatively, type HttpLike} from '../utils/deleteUserViaFlow';

/**
 * Custom hook to delete a user by ID.
 *
 * How the deletion runs is an option rather than a property of the deployment. `flow` runs it
 * through the configured administration flow, which revokes the user's grants and ends their
 * sessions before the record is removed; `native` calls the endpoint directly, which suits a
 * console whose deployment holds configuration only and has nothing live to clean up. A console
 * may also supply a function of its own.
 *
 * This package never asks which plane it is on. It asks how this operation runs, and the answer
 * comes from whatever the console declared.
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
      const client = http as unknown as HttpLike;

      if (typeof mode === 'function') {
        await mode(userId);
        return;
      }
      if (mode === AdministrationModes.NATIVE) {
        await deleteUserNatively(client, getServerUrl(), userId);
        return;
      }
      await deleteUser(client, getServerUrl(), userId);
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
