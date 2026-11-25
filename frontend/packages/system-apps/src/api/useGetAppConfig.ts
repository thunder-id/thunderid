// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AppConfigQueryKeys, appConfigPath} from '../constants/app-config-api';
import type {AppConfig, AppType} from '../models/app-config';

/**
 * Fetches the configuration of an app from its server-config section. The server fills in the app's
 * defaults for anything not set, so the result always describes every page of the app.
 *
 * @param type - The app whose configuration to fetch.
 * @param options - Query options. Set `enabled` to `false` to wait, for example until the user signs in.
 * @returns TanStack Query result object containing the configuration.
 *
 * @public
 */
export default function useGetAppConfig(type: AppType, options: {enabled?: boolean} = {}): UseQueryResult<AppConfig> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AppConfig>({
    queryKey: [AppConfigQueryKeys.APP_CONFIG, type],
    queryFn: async (): Promise<AppConfig> => {
      const response: {data: {merged: AppConfig}} = await http.request({
        url: `${getServerUrl()}${appConfigPath(type)}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.merged;
    },
    enabled: options.enabled ?? true,
  });
}
