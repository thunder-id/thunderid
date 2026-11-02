// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {AccountAccessConfigResponse} from '../models/accountAccess';

/**
 * Fetches the account-lockout server-config section.
 *
 * @returns TanStack Query result containing the section's three layers.
 *
 * @public
 */
export default function useGetAccountAccessConfig(): UseQueryResult<AccountAccessConfigResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AccountAccessConfigResponse>({
    queryKey: [SettingsQueryKeys.SERVER_CONFIG, SettingsQueryKeys.ACCOUNT_ACCESS],
    queryFn: async (): Promise<AccountAccessConfigResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: AccountAccessConfigResponse;
      } = await http.request({
        url: `${serverUrl}/server-config/${SettingsQueryKeys.ACCOUNT_ACCESS}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
