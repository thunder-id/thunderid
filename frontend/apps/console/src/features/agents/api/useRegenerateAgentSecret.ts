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
import type {Agent, AgentInboundAuthConfig} from '../models/agent';

export interface RegenerateAgentSecretVariables {
  agentId: string;
}

export interface RegenerateAgentSecretResult {
  agent: Agent;
  clientSecret: string;
}

/**
 * Generates a cryptographically secure OAuth 2.0 client secret matching the backend's
 * `GenerateOAuth2ClientSecret()` (32 random bytes encoded as base64url, no padding).
 */
function generateClientSecret(): string {
  const bytes = new Uint8Array(32);
  crypto.getRandomValues(bytes);
  return btoa(String.fromCharCode(...bytes))
    .replace(/\+/g, '-')
    .replace(/\//g, '_')
    .replace(/=/g, '');
}

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
      const serverUrl: string = getServerUrl();

      // Step 1: Fetch the current agent
      const getResponse: {data: Agent} = await http.request({
        url: `${serverUrl}/agents/${agentId}`,
        method: 'GET',
        headers: {'Content-Type': 'application/json'},
      } as unknown as Parameters<typeof http.request>[0]);

      const currentAgent = getResponse.data;

      // Step 2: Generate a new client secret
      const newClientSecret = generateClientSecret();

      // Step 3: Build the updated inbound auth config without mutating cached data
      const existingInboundAuth = currentAgent.inboundAuthConfig ?? [];
      const hasOAuth2 = existingInboundAuth.some((c: AgentInboundAuthConfig) => c.type === 'oauth2');

      if (!hasOAuth2) {
        throw new Error('Agent does not have an OAuth2 configuration. Cannot regenerate client secret.');
      }

      const updatedInboundAuth: AgentInboundAuthConfig[] = existingInboundAuth.map((c: AgentInboundAuthConfig) =>
        c.type === 'oauth2'
          ? {...c, config: {...(c.config ?? {grantTypes: [], responseTypes: []}), clientSecret: newClientSecret}}
          : c,
      );

      // Step 4: Strip server-generated fields and PUT
      const {id: _id, clientId: _clientId, ...rest} = currentAgent;
      void _id;
      void _clientId;
      const updateRequest = {...rest, inboundAuthConfig: updatedInboundAuth};

      const updateResponse: {data: Agent} = await http.request({
        url: `${serverUrl}/agents/${agentId}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: updateRequest,
      } as unknown as Parameters<typeof http.request>[0]);

      return {
        agent: updateResponse.data,
        clientSecret: newClientSecret,
      };
    },
    onSuccess: (_data, variables) => {
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENT, variables.agentId]}).catch(() => undefined);
      queryClient.invalidateQueries({queryKey: [AgentQueryKeys.AGENTS]}).catch(() => undefined);
      showToast(t('regenerateSecret.snackbar.success', 'Client secret regenerated successfully'), 'success');
    },
    onError: (error) => {
      showToast(getErrorMessage(error, t, 'regenerateSecret.dialog.error'), 'error');
    },
  });
}
