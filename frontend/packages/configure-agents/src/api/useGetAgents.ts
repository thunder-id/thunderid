// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {AgentListResponse} from '../models/agent';

export interface UseGetAgentsParams {
  limit?: number;
  offset?: number;
}

export default function useGetAgents(params?: UseGetAgentsParams): UseQueryResult<AgentListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<AgentListResponse>({
    queryKey: [AgentQueryKeys.AGENTS, {limit, offset}],
    queryFn: async (): Promise<AgentListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });

      const response: {data: AgentListResponse} = await http.request({
        url: `${serverUrl}/agents?${queryParams.toString()}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
