// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import EnvironmentVariableQueryKeys from '../constants/environment-variable-query-keys';
import type {EnvironmentVariable} from '../models/environment-variable';

export default function useGetEnvironmentVariable(envId: string, id: string): UseQueryResult<EnvironmentVariable> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<EnvironmentVariable>({
    queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLE, envId, id],
    enabled: Boolean(envId) && Boolean(id),
    queryFn: async (): Promise<EnvironmentVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: EnvironmentVariable} = await http.request({
        url: `${serverUrl}/environments/${envId}/variables/${id}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
