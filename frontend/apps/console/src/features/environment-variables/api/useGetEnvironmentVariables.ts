// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import EnvironmentVariableQueryKeys from '../constants/environment-variable-query-keys';
import type {EnvironmentVariableListParams, EnvironmentVariableListResponse} from '../models/environment-variable';

export default function useGetEnvironmentVariables(
  envId: string,
  params?: EnvironmentVariableListParams,
): UseQueryResult<EnvironmentVariableListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<EnvironmentVariableListResponse>({
    queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLES, envId, {limit, offset}],
    enabled: Boolean(envId),
    queryFn: async (): Promise<EnvironmentVariableListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams = new URLSearchParams({limit: limit.toString(), offset: offset.toString()});
      const response: {
        data: EnvironmentVariableListResponse;
      } = await http.request({
        url: `${serverUrl}/environments/${envId}/variables?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
