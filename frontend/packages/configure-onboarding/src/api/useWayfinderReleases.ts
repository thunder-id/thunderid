// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import type {ReleasesData} from '../models/download-assets';

export default function useWayfinderReleases(releasesUrl: string): UseQueryResult<ReleasesData, Error> {
  return useQuery<ReleasesData, Error>({
    queryKey: ['wayfinder-releases', releasesUrl],
    queryFn: async (): Promise<ReleasesData> => {
      const response = await fetch(releasesUrl);
      if (!response.ok) throw new Error(`Failed to fetch releases: ${response.status}`);
      return response.json() as Promise<ReleasesData>;
    },
    retry: 1,
    staleTime: 5 * 60 * 1000,
  });
}
