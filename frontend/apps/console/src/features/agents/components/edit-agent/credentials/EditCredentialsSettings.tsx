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
import type {JSX} from 'react';
import CertificateSection from './CertificateSection';
import ClientIdSection from './ClientIdSection';
import ClientSecretSection from './ClientSecretSection';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

interface EditCredentialsSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  copiedField: string | null;
  onCopyToClipboard: (text: string, fieldName: string) => Promise<void>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function EditCredentialsSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  copiedField,
  onCopyToClipboard,
  onFieldChange,
}: EditCredentialsSettingsProps): JSX.Element {
  const handleOAuth2ConfigChange = (updates: Partial<OAuthAgentConfig>) => {
    const currentInboundAuth = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  return (
    <Stack spacing={3}>
      <ClientIdSection oauth2Config={oauth2Config} copiedField={copiedField} onCopyToClipboard={onCopyToClipboard} />
      <ClientSecretSection agentId={agent.id} oauth2Config={oauth2Config} disabled={agent.isReadOnly} />
      <CertificateSection
        certificate={oauth2Config?.certificate}
        onCertificateChange={(cert) => handleOAuth2ConfigChange({certificate: cert})}
        required={oauth2Config?.tokenEndpointAuthMethod === 'private_key_jwt'}
        disabled={agent.isReadOnly}
      />
    </Stack>
  );
}
