// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {ConfigurationVersion} from '../models/gateway';

/**
 * Lists the captured configuration versions, newest first, without their content.
 */
export default function useGetConfigurationVersions(): UseQueryResult<ConfigurationVersion[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConfigurationVersion[]>({
    queryKey: [GatewayQueryKeys.CONFIGURATION_VERSIONS],
    queryFn: async (): Promise<ConfigurationVersion[]> => {
      const response: {data: ConfigurationVersion[] | null} = await http.request({
        url: `${getServerUrl()}/configuration-versions`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data ?? [];
    },
  });
}
