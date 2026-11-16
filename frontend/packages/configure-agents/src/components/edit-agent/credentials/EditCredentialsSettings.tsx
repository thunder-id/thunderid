// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import CertificateSection from './CertificateSection';
import ClientIdSection from './ClientIdSection';
import ClientSecretSection from './ClientSecretSection';
import type {AgentCredentialField} from './CredentialResetDialog';
import CredentialResetSection from './CredentialResetSection';
import {PERSONAL_AGENT_KIND, type Agent, type OAuthAgentConfig} from '../../../models/agent';

interface EditCredentialsSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  /** The credentials the agent type declares, such as a password. None, or a personal agent, shows no reset. */
  credentialFields?: AgentCredentialField[];
}

export default function EditCredentialsSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  credentialFields = [],
}: EditCredentialsSettingsProps): JSX.Element {
  const handleOAuth2ConfigChange = (updates: Partial<OAuthAgentConfig>) => {
    const currentInboundAuth = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  // An encrypted ID token is encrypted to this certificate, so removing it while such a format is
  // selected would produce an invalid config. Used to block that removal. Agents have no UserInfo.
  const idTokenResponseType = oauth2Config?.token?.idToken?.responseType;
  const encryptionDependsOnCert = idTokenResponseType === 'JWE' || idTokenResponseType === 'NESTED_JWT';

  // Only the owner sets a personal agent's credentials, so the administrator's reset is not offered for one.
  const canResetCredentials = agent.kind !== PERSONAL_AGENT_KIND;

  return (
    <Stack spacing={3}>
      {oauth2Config && (
        <>
          <ClientIdSection oauth2Config={oauth2Config} />
          <ClientSecretSection agentId={agent.id} oauth2Config={oauth2Config} disabled={agent.isReadOnly} />
          <CertificateSection
            certificate={oauth2Config.certificate}
            onCertificateChange={(cert) => handleOAuth2ConfigChange({certificate: cert})}
            required={oauth2Config.tokenEndpointAuthMethod === 'private_key_jwt'}
            encryptionDependsOnCert={encryptionDependsOnCert}
            disabled={agent.isReadOnly}
          />
        </>
      )}
      {canResetCredentials && (
        <CredentialResetSection agentId={agent.id} fields={credentialFields} disabled={agent.isReadOnly} />
      )}
    </Stack>
  );
}
