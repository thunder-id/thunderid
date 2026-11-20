// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';

export interface UpdateAgentCredentialsVariables {
  agentId: string;
  data: {
    credentials: Record<string, string>;
  };
}

/**
 * Sets new values for an agent's credentials through `POST /agents/{id}/update-credentials`. Replacing a
 * credential the agent already holds revokes its tokens and ends its sessions, as a user's reset does.
 */
export default function useUpdateAgentCredentials(): UseMutationResult<void, Error, UpdateAgentCredentialsVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {t} = useTranslation('agents');
  const {showToast} = useToast();

  return useMutation<void, Error, UpdateAgentCredentialsVariables>({
    mutationFn: async ({agentId, data}: UpdateAgentCredentialsVariables): Promise<void> => {
      await http.request({
        url: `${getServerUrl()}/agents/${encodeURIComponent(agentId)}/update-credentials`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      showToast(t('updateCredentials.success', 'Credentials updated successfully'), 'success');
    },
  });
}
