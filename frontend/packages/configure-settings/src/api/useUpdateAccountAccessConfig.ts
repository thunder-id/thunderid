// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {AccountAccessConfigResponse, AccountAccessValue} from '../models/accountAccess';

/**
 * Variables for the {@link useUpdateAccountAccessConfig} mutation.
 *
 * @public
 */
export interface UpdateAccountAccessConfigVariables {
  /**
   * The writable value to persist. It replaces the writable layer whole, so it must carry everything
   * this deployment has changed, not only what this save changed.
   */
  data: AccountAccessValue;
}

/**
 * Updates the writable layer of the account-lockout server-config section.
 *
 * @returns TanStack Query mutation for updating the lockout policy.
 *
 * @public
 */
export default function useUpdateAccountAccessConfig(): UseMutationResult<
  AccountAccessConfigResponse,
  Error,
  UpdateAccountAccessConfigVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<AccountAccessConfigResponse, Error, UpdateAccountAccessConfigVariables>({
    mutationFn: async ({data}: UpdateAccountAccessConfigVariables): Promise<AccountAccessConfigResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: AccountAccessConfigResponse;
      } = await http.request({
        url: `${serverUrl}/server-config/${SettingsQueryKeys.ACCOUNT_ACCESS}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data) => {
      // PUT returns the recomputed layers, so keep the read model in sync without a refetch.
      queryClient.setQueryData([SettingsQueryKeys.SERVER_CONFIG, SettingsQueryKeys.ACCOUNT_ACCESS], data);
      showToast(t('settings:accountLock.save.success', 'Account security settings updated.'), 'success');
    },
  });
}
