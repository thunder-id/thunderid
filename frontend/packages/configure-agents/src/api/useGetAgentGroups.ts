// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {AgentGroupListResponse} from '../models/agent';

export interface UseGetAgentGroupsParams {
  limit?: number;
  offset?: number;
}

export default function useGetAgentGroups(
  agentId: string,
  params?: UseGetAgentGroupsParams,
): UseQueryResult<AgentGroupListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<AgentGroupListResponse>({
    queryKey: [AgentQueryKeys.AGENT_GROUPS, agentId, {limit, offset}],
    queryFn: async (): Promise<AgentGroupListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      const response: {data: AgentGroupListResponse} = await http.request({
        url: `${serverUrl}/agents/${agentId}/groups?${queryParams.toString()}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(agentId),
  });
}
