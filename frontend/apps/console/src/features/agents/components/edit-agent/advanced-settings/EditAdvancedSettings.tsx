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

import {Stack} from '@wso2/oxygen-ui';
import AllowedUserTypesSection from './AllowedUserTypesSection';
import OperationModesSection from './OperationModesSection';
import OwnerSection from './OwnerSection';
import SecuritySection from './SecuritySection';
import TokenEndpointAuthMethodSection from './TokenEndpointAuthMethodSection';
import AudienceSection from '../../../../applications/components/edit-application/advanced-settings/AudienceSection';
import type {OAuth2Config} from '../../../../applications/models/oauth';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';

interface EditAdvancedSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function EditAdvancedSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
}: EditAdvancedSettingsProps) {
  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleDefaultAudienceChange = (audience: string) => {
    handleOAuth2ConfigChange({
      token: {
        ...oauth2Config?.token,
        accessToken: {...oauth2Config?.token?.accessToken, defaultAudience: audience},
      } as OAuth2Config['token'],
    });
  };

  return (
    <Stack spacing={3}>
      <OwnerSection agent={agent} editedAgent={editedAgent} onFieldChange={onFieldChange} />
      <AllowedUserTypesSection
        agent={agent}
        editedAgent={editedAgent}
        oauth2Config={oauth2Config}
        onFieldChange={onFieldChange}
      />
      <OperationModesSection
        oauth2Config={oauth2Config}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={agent.isReadOnly}
      />
      <TokenEndpointAuthMethodSection
        oauth2Config={oauth2Config}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={agent.isReadOnly}
      />
      <SecuritySection
        oauth2Config={oauth2Config}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={agent.isReadOnly}
      />
      {oauth2Config && (
        <AudienceSection
          audience={oauth2Config.token?.accessToken?.defaultAudience ?? ''}
          onAudienceChange={handleDefaultAudienceChange}
          entityLabel="agent"
          disabled={agent.isReadOnly}
        />
      )}
    </Stack>
  );
}
