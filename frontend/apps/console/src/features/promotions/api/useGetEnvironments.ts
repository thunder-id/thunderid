// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {EnvironmentListResponse} from '../models/promotion';

/**
 * Lists the environments in the promotion chain, ordered by rank.
 */
export default function useGetEnvironments(): UseQueryResult<EnvironmentListResponse> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();

  return useQuery<EnvironmentListResponse>({
    queryKey: [PromotionQueryKeys.ENVIRONMENTS],
    enabled: Boolean(baseUrl),
    queryFn: async (): Promise<EnvironmentListResponse> => {
      const response: {data: EnvironmentListResponse} = await http.request({
        url: `${baseUrl}/environments`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
