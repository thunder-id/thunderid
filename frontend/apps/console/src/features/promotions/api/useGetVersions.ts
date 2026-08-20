// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {VersionListResponse} from '../models/promotion';

/**
 * Lists an environment's retained configuration versions, newest first.
 */
export default function useGetVersions(envId: string): UseQueryResult<VersionListResponse> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();

  return useQuery<VersionListResponse>({
    queryKey: [PromotionQueryKeys.VERSIONS, envId],
    enabled: Boolean(baseUrl) && Boolean(envId),
    queryFn: async (): Promise<VersionListResponse> => {
      const response: {data: VersionListResponse} = await http.request({
        url: `${baseUrl}/environments/${envId}/versions`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
