// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import ApplicationQueryKeys from '../constants/application-query-keys';
import parseQuickstartGuide, {type QuickstartGuideContent} from '../utils/parseQuickstartGuide';

/**
 * Fetches a quickstart guide from the docs site and returns its steps and its sample app.
 *
 * The docs site publishes a Markdown export of every page next to the page itself, at the page's
 * URL with a `.md` suffix. Reading that keeps the docs the single source of the guide's content.
 *
 * @param docsUrl - The URL of the quickstart page on the docs site
 * @param section - The anchor of the section that holds the steps, when it is not the whole page
 * @param variants - The choices to show the page for, by kind, when it is written for several
 * @returns TanStack Query result holding the steps and the sample, or null when the page is not a
 * quickstart the Console can show inline
 *
 * @public
 */
export default function useGetQuickstartGuide(
  docsUrl: string | undefined,
  section?: string,
  variants?: Record<string, string>,
): UseQueryResult<QuickstartGuideContent | null> {
  return useQuery<QuickstartGuideContent | null>({
    queryKey: [ApplicationQueryKeys.QUICKSTART_GUIDE, docsUrl, section, variants],
    queryFn: async (): Promise<QuickstartGuideContent | null> => {
      const response = await fetch(`${docsUrl!.replace(/\/+$/, '')}.md`);

      if (!response.ok) {
        throw new Error(`Failed to fetch the quickstart guide: ${response.status}`);
      }

      return parseQuickstartGuide(await response.text(), section, variants);
    },
    enabled: Boolean(docsUrl),
    staleTime: Infinity,
    retry: false,
  });
}
