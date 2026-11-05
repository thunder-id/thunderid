// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AGENT_TYPES_PAGE_SIZE} from '../constants/agentTypeConstants';
import AgentTypeQueryKeys from '../constants/agentTypeQueryKeys';
import type {AgentTypeListItem} from '../models/agent-type';
import type {AgentTypeListResponse} from '../models/responses';

/**
 * Fetches every agent type, following pagination to the end.
 *
 * Use it wherever a type is looked up by handle or every type is listed, so the result never
 * depends on how many types fit on the first page.
 *
 * @param options - Optional query options
 * @param options.enabled - Set to false to skip the request
 * @returns TanStack Query result holding all agent types, with their display data
 *
 * @example
 * ```tsx
 * const {data: agentTypes} = useGetAllAgentTypes();
 * const agentType = agentTypes?.find((type) => type.handle === agent.type);
 * ```
 *
 * @public
 */
export default function useGetAllAgentTypes(options?: {enabled?: boolean}): UseQueryResult<AgentTypeListItem[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {enabled = true} = options ?? {};

  return useQuery<AgentTypeListItem[]>({
    queryKey: [AgentTypeQueryKeys.AGENT_TYPES, 'all'],
    queryFn: async (): Promise<AgentTypeListItem[]> => {
      const serverUrl: string = getServerUrl();
      const agentTypes: AgentTypeListItem[] = [];

      for (;;) {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: AGENT_TYPES_PAGE_SIZE.toString(),
          offset: agentTypes.length.toString(),
          include: 'display',
        });

        const response: {
          data: AgentTypeListResponse;
        } = await http.request({
          url: `${serverUrl}/agent-types?${queryParams.toString()}`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        const page: AgentTypeListItem[] = response.data.types ?? [];
        agentTypes.push(...page);

        if (page.length === 0 || agentTypes.length >= response.data.totalResults) {
          return agentTypes;
        }
      }
    },
    enabled,
  });
}
