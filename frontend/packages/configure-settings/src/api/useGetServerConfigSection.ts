// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {ServerConfigLayers} from '../models/responses';

/**
 * Fetches one server-config section.
 *
 * @param name - The section name, such as `session`
 * @returns TanStack Query result containing the section's layers.
 *
 * @public
 */
export default function useGetServerConfigSection<T>(name: string): UseQueryResult<ServerConfigLayers<T>> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ServerConfigLayers<T>>({
    queryKey: [SettingsQueryKeys.SERVER_CONFIG, name],
    queryFn: async (): Promise<ServerConfigLayers<T>> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ServerConfigLayers<T>;
      } = await http.request({
        url: `${serverUrl}/server-config/${name}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
