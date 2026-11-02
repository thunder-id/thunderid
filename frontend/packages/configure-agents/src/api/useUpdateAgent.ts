// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {Agent, UpdateAgentRequest} from '../models/agent';

interface UpdateAgentParams {
  agentId: string;
  data: UpdateAgentRequest;
}

export default function useUpdateAgent(): UseMutationResult<Agent, Error, UpdateAgentParams> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('agents');
  const {showToast} = useToast();

  return useMutation<Agent, Error, UpdateAgentParams>({
    mutationFn: async ({agentId, data}: UpdateAgentParams): Promise<Agent> => {
      const serverUrl = getServerUrl();
      const response: {data: Agent} = await http.request({
        url: `${serverUrl}/agents/${agentId}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_, {agentId}) => {
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENT, agentId]}).catch(() => undefined);
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENTS]}).catch(() => undefined);
      showToast(t('update.success'), 'success');
    },
  });
}
