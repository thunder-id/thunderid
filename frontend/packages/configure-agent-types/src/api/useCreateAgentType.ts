// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentTypeQueryKeys from '../constants/agentTypeQueryKeys';
import type {ApiAgentType} from '../models/agent-type';
import type {CreateAgentTypeRequest} from '../models/requests';

/**
 * Custom React hook to create a new agent type in the server.
 *
 * Failures are left to the caller, which renders them next to the form that submitted them.
 *
 * @returns TanStack Query mutation object for creating agent types
 *
 * @example
 * ```tsx
 * const createAgentType = useCreateAgentType();
 * await createAgentType.mutateAsync({handle: 'assistant', displayName: 'Assistant', ouId, schema});
 * ```
 *
 * @public
 */
export default function useCreateAgentType(): UseMutationResult<ApiAgentType, Error, CreateAgentTypeRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('agentTypes');
  const {showToast} = useToast();

  return useMutation<ApiAgentType, Error, CreateAgentTypeRequest>({
    mutationFn: async (requestData: CreateAgentTypeRequest): Promise<ApiAgentType> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ApiAgentType;
      } = await http.request({
        url: `${serverUrl}/agent-types`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: requestData,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('create.success', 'Agent type created successfully.'), 'success');
    },
  });
}
