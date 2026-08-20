// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {VariableStatus} from '../models/promotion';

/**
 * Reports which placeholders an environment's next apply would fail to resolve. An unresolved
 * placeholder is not an import error: the field simply renders empty, so it has to be surfaced before
 * the apply rather than diagnosed afterwards.
 */
export default function useCheckVariables(envId: string, version?: string): UseQueryResult<VariableStatus> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();

  return useQuery<VariableStatus>({
    queryKey: [PromotionQueryKeys.VARIABLES, envId, version ?? 'latest'],
    enabled: Boolean(baseUrl) && Boolean(envId),
    queryFn: async (): Promise<VariableStatus> => {
      const params = new URLSearchParams();
      if (version) {
        params.set('version', version);
      }
      const query: string = params.toString() ? `?${params.toString()}` : '';
      const response: {data: VariableStatus} = await http.request({
        url: `${baseUrl}/environments/${envId}/variables${query}`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    retry: false,
    // The control plane's values are read live, so a variable edited in another screen must show up
    // here without a manual refresh.
    staleTime: 0,
    refetchOnMount: 'always',
  });
}
