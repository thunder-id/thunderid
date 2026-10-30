// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';

interface CandidatePage<T> {
  totalResults: number;
  count: number;
  [key: string]: T[] | number;
}

interface UseGetAllCandidatesOptions {
  path: string;
  itemsKey: string;
  enabled: boolean;
  /** Filter expression, for example `ouId eq "<id>"`. */
  filter?: string;
}

export default function useGetAllCandidates<T>({
  path,
  itemsKey,
  enabled,
  filter = undefined,
}: UseGetAllCandidatesOptions): UseQueryResult<T[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<T[]>({
    queryKey: ['group-add-member-candidates', path, filter],
    queryFn: async (): Promise<T[]> => {
      const serverUrl: string = getServerUrl();
      const candidates: T[] = [];
      const limit = 100;
      let totalResults = 0;
      let offset = 0;

      do {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: limit.toString(),
          offset: offset.toString(),
          include: 'display',
        });
        if (filter) {
          queryParams.append('filter', filter);
        }
        const response: {data: CandidatePage<T>} = await http.request({
          url: `${serverUrl}${path}?${queryParams.toString()}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);
        const page = response.data;
        const pageCandidates = page[itemsKey];

        if (!Array.isArray(pageCandidates)) {
          throw new Error(`Invalid candidate response for ${itemsKey}`);
        }

        candidates.push(...pageCandidates);
        totalResults = page.totalResults;
        offset += page.count;

        if (page.count === 0) break;
      } while (offset < totalResults);

      return candidates;
    },
    enabled,
  });
}
