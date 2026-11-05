// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentTypeQueryKeys from '../constants/agentTypeQueryKeys';

/**
 * Custom React hook to delete an agent type from the server.
 *
 * The deleted type's detail query is marked stale rather than removed: the agent type page that
 * starts the delete still observes it, and removing an observed query makes React Query fetch the
 * deleted type again.
 *
 * @returns TanStack Query mutation object for deleting agent types, keyed by agent type ID
 *
 * @example
 * ```tsx
 * const deleteAgentType = useDeleteAgentType();
 * deleteAgentType.mutate(agentTypeId);
 * ```
 *
 * @public
 */
export default function useDeleteAgentType(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('agentTypes');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (agentTypeId: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/agent-types/${agentTypeId}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, agentTypeId) => {
      queryClient
        .invalidateQueries({queryKey: [AgentTypeQueryKeys.AGENT_TYPE, agentTypeId], refetchType: 'none'})
        .catch(() => {
          // Ignore invalidation errors
        });
      queryClient.invalidateQueries({queryKey: [AgentTypeQueryKeys.AGENT_TYPES]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success', 'Agent type deleted successfully.'), 'success');
    },
  });
}
