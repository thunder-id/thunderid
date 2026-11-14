// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import ApplicationQueryKeys from '../constants/application-query-keys';
import parseQuickstartGuide, {type QuickstartGuideStep} from '../utils/parseQuickstartGuide';

/**
 * Fetches a quickstart guide from the docs site and returns its steps.
 *
 * The docs site publishes a Markdown export of every page next to the page itself, at the page's
 * URL with a `.md` suffix. Reading that keeps the docs the single source of the guide's content.
 *
 * @param docsUrl - The URL of the quickstart page on the docs site
 * @returns TanStack Query result holding the steps, or null when the page is not a quickstart the
 * Console can show inline
 *
 * @public
 */
export default function useGetQuickstartGuide(
  docsUrl: string | undefined,
): UseQueryResult<QuickstartGuideStep[] | null> {
  return useQuery<QuickstartGuideStep[] | null>({
    queryKey: [ApplicationQueryKeys.QUICKSTART_GUIDE, docsUrl],
    queryFn: async (): Promise<QuickstartGuideStep[] | null> => {
      const response = await fetch(`${docsUrl!.replace(/\/+$/, '')}.md`);

      if (!response.ok) {
        throw new Error(`Failed to fetch the quickstart guide: ${response.status}`);
      }

      return parseQuickstartGuide(await response.text());
    },
    enabled: Boolean(docsUrl),
    staleTime: Infinity,
    retry: false,
  });
}
