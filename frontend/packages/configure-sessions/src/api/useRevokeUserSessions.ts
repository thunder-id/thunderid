// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import SessionQueryKeys from '../constants/session-query-keys';

/**
 * Custom hook to revoke every session of a user.
 *
 * @returns TanStack Query mutation object for revoking a user's sessions, called with the user ID
 */
export default function useRevokeUserSessions(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('sessions');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (userId: string): Promise<void> => {
      await http.request({
        url: `${getServerUrl()}/sessions?${new URLSearchParams({userId}).toString()}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [SessionQueryKeys.SESSIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('revokeAll.success', 'All sessions revoked'), 'success');
    },
  });
}
