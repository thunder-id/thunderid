// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

// TEMPORARY: mirrors apps/console/src/features/agents/api/useGetAgents.ts. The agents feature
// hasn't been extracted into @thunderid/configure-agents yet; once it is, this file should be
// deleted and AddAssignmentDialog should import from that package instead.

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {AgentListResponse} from './agent';
import AgentQueryKeys from './agent-query-keys';

export interface UseGetAgentsParams {
  limit?: number;
  offset?: number;
  filter?: string;
}

export default function useGetAgents(params?: UseGetAgentsParams): UseQueryResult<AgentListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, filter} = params ?? {};

  return useQuery<AgentListResponse>({
    queryKey: [AgentQueryKeys.AGENTS, {limit, offset, filter}],
    queryFn: async (): Promise<AgentListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });
      if (filter) {
        queryParams.append('filter', filter);
      }

      const response: {data: AgentListResponse} = await http.request({
        url: `${serverUrl}/agents?${queryParams.toString()}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
