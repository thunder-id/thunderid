// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import ApplicationQueryKeys from '../constants/application-query-keys';

/**
 * The released archive of a sample app.
 *
 * @public
 */
export interface SampleRelease {
  /** The URL of the archive to download */
  downloadUrl: string;
  /** The archive's size, formatted for display (e.g. "48 KB") */
  sizeLabel?: string;
  /** The released version of the sample (e.g. "1.0.0") */
  version?: string;
}

interface EcosystemRelease {
  type: 'sdk' | 'sample';
  packageId: string;
  latestVersion?: string;
  assets?: {downloadUrl: string; sizeLabel?: string}[];
}

/**
 * Finds the latest released archive of a sample app in the ecosystem releases feed, the same feed
 * the docs site offers sample downloads from.
 *
 * @param releasesUrl - The URL of the ecosystem releases feed
 * @param sampleId - The sample's ID in the feed (e.g. "quickstarts/react")
 * @returns TanStack Query result holding the archive, or null when the sample has no released archive
 *
 * @public
 */
export default function useGetSampleRelease(
  releasesUrl: string | undefined,
  sampleId: string | undefined,
): UseQueryResult<SampleRelease | null> {
  return useQuery<SampleRelease | null>({
    queryKey: [ApplicationQueryKeys.SAMPLE_RELEASE, releasesUrl, sampleId],
    queryFn: async (): Promise<SampleRelease | null> => {
      const response = await fetch(releasesUrl!);

      if (!response.ok) {
        throw new Error(`Failed to fetch the ecosystem releases: ${response.status}`);
      }

      const {releases} = (await response.json()) as {releases?: EcosystemRelease[]};
      const release = releases?.find((entry) => entry.type === 'sample' && entry.packageId === sampleId);
      const archive = release?.assets?.find((asset) => asset.downloadUrl.endsWith('.zip'));

      return archive
        ? {downloadUrl: archive.downloadUrl, sizeLabel: archive.sizeLabel, version: release?.latestVersion}
        : null;
    },
    enabled: Boolean(releasesUrl && sampleId),
    staleTime: Infinity,
    retry: false,
  });
}
