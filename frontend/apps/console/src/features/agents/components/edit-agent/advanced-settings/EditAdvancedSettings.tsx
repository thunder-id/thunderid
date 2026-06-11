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
import CertificateSection from './CertificateSection';
import OAuth2ConfigSection from './OAuth2ConfigSection';
import RedirectURIsSection from './RedirectURIsSection';
import type {OAuth2Config} from '../../../../applications/models/oauth';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';

interface EditAdvancedSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  /**
   * Bubbled up from the redirect-URI section to the page-level Save guard.
   */
  onValidationChange?: (hasErrors: boolean) => void;
}

export default function EditAdvancedSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
}: EditAdvancedSettingsProps) {
  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  return (
    <Stack spacing={3}>
      <OAuth2ConfigSection
        oauth2Config={oauth2Config}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        disabled={agent.isReadOnly}
      />
      <RedirectURIsSection
        oauth2Config={oauth2Config}
        onOAuth2ConfigChange={handleOAuth2ConfigChange}
        onValidationChange={onValidationChange}
        disabled={agent.isReadOnly}
      />
      <CertificateSection
        certificate={oauth2Config?.certificate}
        onCertificateChange={(cert) => handleOAuth2ConfigChange({certificate: cert})}
        required={oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt'}
        disabled={agent.isReadOnly}
      />
    </Stack>
  );
}
