// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig, useProjectScope} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {AgentListResponse} from '../models/agent';

export interface UseGetAgentsParams {
  limit?: number;
  offset?: number;
  /** Lists only the agents of this kind, such as `personal-assistant`. */
  kind?: string;
}

/**
 * The agents, a page at a time. Inside a `ProjectScopeProvider` with a selected project, only that
 * project's agents are listed, and a kind narrows the list to the agents of that kind.
 */
export default function useGetAgents(params?: UseGetAgentsParams): UseQueryResult<AgentListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, kind} = params ?? {};
  const {projectId} = useProjectScope();

  return useQuery<AgentListResponse>({
    queryKey: [AgentQueryKeys.AGENTS, {limit, offset, projectId, kind}],
    queryFn: async (): Promise<AgentListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });
      if (projectId) {
        queryParams.append('projectId', projectId);
      }
      if (kind) {
        queryParams.append('filter', `kind eq "${kind}"`);
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
