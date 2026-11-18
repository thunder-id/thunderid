// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {CspConfigResponse} from '../models/responses';

/**
 * Fetches the CSP server-config section.
 *
 * @returns TanStack Query result containing the CSP config layers.
 *
 * @public
 */
export default function useGetCspConfig(): UseQueryResult<CspConfigResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<CspConfigResponse>({
    queryKey: [SettingsQueryKeys.SERVER_CONFIG, SettingsQueryKeys.CSP],
    queryFn: async (): Promise<CspConfigResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: CspConfigResponse;
      } = await http.request({
        url: `${serverUrl}/server-config/csp`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
