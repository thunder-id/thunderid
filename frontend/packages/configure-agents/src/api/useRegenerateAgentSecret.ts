// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import AgentQueryKeys from '../constants/agent-query-keys';

export interface RegenerateAgentSecretVariables {
  agentId: string;
}

export interface RegenerateAgentSecretResult {
  /**
   * The new client secret. This is the only time it can be read.
   */
  clientSecret: string;
  /**
   * Set when the secret is stored but revoking the tokens issued while it was replaced failed.
   * Regenerating again revokes them.
   */
  revocationIncomplete?: boolean;
}

/**
 * Regenerates an agent's client secret through `POST /agents/{id}/client-secret/regenerate`. The server
 * generates the new secret, revokes the tokens issued under the old one, and returns the new secret once.
 */
export default function useRegenerateAgentSecret(): UseMutationResult<
  RegenerateAgentSecretResult,
  Error,
  RegenerateAgentSecretVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation('agents');
  const {showToast} = useToast();

  return useMutation<RegenerateAgentSecretResult, Error, RegenerateAgentSecretVariables>({
    mutationFn: async ({agentId}: RegenerateAgentSecretVariables): Promise<RegenerateAgentSecretResult> => {
      const response: {data: RegenerateAgentSecretResult} = await http.request({
        url: `${getServerUrl()}/agents/${encodeURIComponent(agentId)}/client-secret/regenerate`,
        method: 'POST',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data, variables) => {
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENT, variables.agentId]}).catch(() => undefined);
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENTS]}).catch(() => undefined);
      if (data.revocationIncomplete) {
        showToast(
          t(
            'regenerateSecret.snackbar.revocationIncomplete',
            'Client secret regenerated, but tokens issued while it was replaced may still be valid. Regenerate it again to revoke them.',
          ),
          'warning',
        );
      } else {
        showToast(t('regenerateSecret.snackbar.success', 'Client secret regenerated successfully'), 'success');
      }
    },
  });
}
