// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentQueryKeys from '../constants/agent-query-keys';
import type {Agent, CreateAgentRequest} from '../models/agent';

/**
 * Creates an agent with the agents management API, without running an onboarding flow.
 */
export default function useCreateAgent(): UseMutationResult<Agent, Error, CreateAgentRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('agents');
  const {showToast} = useToast();

  return useMutation<Agent, Error, CreateAgentRequest>({
    mutationFn: async (data: CreateAgentRequest): Promise<Agent> => {
      const response: {data: Agent} = await http.request({
        url: `${getServerUrl()}/agents`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENTS]}).catch(() => undefined);
      showToast(t('create.success'), 'success');
    },
  });
}
