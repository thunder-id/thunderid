// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import ApplicationQueryKeys from '../constants/application-query-keys';

/**
 * Fetches a coding agent prompt from the docs site, as written, with its placeholders unfilled.
 *
 * @param promptUrl - The URL of the prompt on the docs site
 * @returns TanStack Query result holding the prompt text
 *
 * @public
 */
export default function useGetCodingAgentPrompt(promptUrl: string | undefined): UseQueryResult<string> {
  return useQuery<string>({
    queryKey: [ApplicationQueryKeys.CODING_AGENT_PROMPT, promptUrl],
    queryFn: async (): Promise<string> => {
      const response = await fetch(promptUrl!);

      if (!response.ok) {
        throw new Error(`Failed to fetch the coding agent prompt: ${response.status}`);
      }

      return response.text();
    },
    enabled: Boolean(promptUrl),
    staleTime: Infinity,
    retry: false,
  });
}
