/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import {useTranslation} from 'react-i18next';
import AgentTypeQueryKeys from '../constants/agentTypeQueryKeys';
import type {ApiAgentType} from '../models/agent-type';
import type {UpdateAgentTypeRequest} from '../models/requests';

/**
 * Variables for the {@link useUpdateAgentType} mutation.
 */
export interface UpdateAgentTypeVariables {
  /**
   * The unique identifier of the agent type to update
   */
  agentTypeId: string;
  /**
   * The updated agent type data
   */
  data: UpdateAgentTypeRequest;
}

/**
 * Custom React hook to update an existing agent type in the server.
 *
 * @returns TanStack Query mutation object for updating agent types
 */
export default function useUpdateAgentType(): UseMutationResult<ApiAgentType, Error, UpdateAgentTypeVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('agentTypes');
  const {showToast} = useToast();

  return useMutation<ApiAgentType, Error, UpdateAgentTypeVariables>({
    mutationFn: async ({agentTypeId, data}: UpdateAgentTypeVariables): Promise<ApiAgentType> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ApiAgentType;
      } = await http.request({
        url: `${serverUrl}/agent-types/${agentTypeId}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data: data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({queryKey: [AgentTypeQueryKeys.AGENT_TYPE, variables.agentTypeId]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success'), 'success');
    },
    onError: () => {
      showToast(t('update.error'), 'error');
    },
  });
}
