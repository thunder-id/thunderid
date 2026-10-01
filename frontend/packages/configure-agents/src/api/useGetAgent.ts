// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {Agent} from '../models/agent';

export default function useGetAgent(agentId: string): UseQueryResult<Agent> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Agent>({
    queryKey: [AgentQueryKeys.AGENT, agentId],
    queryFn: async (): Promise<Agent> => {
      const serverUrl = getServerUrl();
      const response: {data: Agent} = await http.request({
        url: `${serverUrl}/agents/${agentId}?include=display`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(agentId),
  });
}
