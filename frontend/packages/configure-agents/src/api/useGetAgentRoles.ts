// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {AgentRoleListResponse} from '../models/agent';

export interface UseGetAgentRolesParams {
  limit?: number;
  offset?: number;
}

export default function useGetAgentRoles(
  agentId: string,
  params?: UseGetAgentRolesParams,
): UseQueryResult<AgentRoleListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<AgentRoleListResponse>({
    queryKey: [AgentQueryKeys.AGENT_ROLES, agentId, {limit, offset}],
    queryFn: async (): Promise<AgentRoleListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      const response: {data: AgentRoleListResponse} = await http.request({
        url: `${serverUrl}/agents/${agentId}/roles?${queryParams.toString()}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(agentId),
  });
}
