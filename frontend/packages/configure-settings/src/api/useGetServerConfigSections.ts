// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SettingsQueryKeys from '../constants/settings-query-keys';

/**
 * Lists the server-config sections this server serves, which decides the settings tabs it offers.
 *
 * @returns TanStack Query result containing the section names.
 *
 * @public
 */
export default function useGetServerConfigSections(): UseQueryResult<string[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<string[]>({
    queryKey: [SettingsQueryKeys.SERVER_CONFIG],
    queryFn: async (): Promise<string[]> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: string[];
      } = await http.request({
        url: `${serverUrl}/server-config`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
