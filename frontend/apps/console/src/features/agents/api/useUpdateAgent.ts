/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {getErrorMessage} from '@thunderid/utils';
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
    onError: (error) => {
      showToast(getErrorMessage(error, t, 'update.error'), 'error');
    },
  });
}
