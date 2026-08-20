// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Diff} from '../models/promotion';

/**
 * Previews what promoting a source environment's version into a target environment would change.
 * This is the diff the user reviews, and selects from, before promoting.
 */
export default function useGetPromotionPreview(fromEnv: string, toEnv: string, version?: string): UseQueryResult<Diff> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();

  return useQuery<Diff>({
    queryKey: [PromotionQueryKeys.PROMOTION_PREVIEW, fromEnv, toEnv, version ?? 'latest'],
    enabled: Boolean(baseUrl) && Boolean(fromEnv) && Boolean(toEnv),
    queryFn: async (): Promise<Diff> => {
      const params = new URLSearchParams({fromEnv, toEnv});
      if (version) {
        params.set('version', version);
      }
      const response: {data: Diff} = await http.request({
        url: `${baseUrl}/promotions/preview?${params.toString()}`,
        method: 'GET',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
