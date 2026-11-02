// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentQueryKeys from '../constants/agent-query-keys';

export default function useDeleteAgent(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('agents');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (agentId: string): Promise<void> => {
      const serverUrl = getServerUrl();
      await http.request({
        url: `${serverUrl}/agents/${agentId}`,
        method: 'DELETE',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, agentId) => {
      queryClient.removeQueries({queryKey: [AgentQueryKeys.AGENT, agentId]});
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENTS]}).catch(() => undefined);
      showToast(t('delete.success', 'Agent deleted successfully'), 'success');
    },
  });
}
