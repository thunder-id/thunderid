// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {CredentialResetSection, type CredentialResetField} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {getErrorMessage} from '@thunderid/utils';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CertificateSection from './CertificateSection';
import ClientIdSection from './ClientIdSection';
import ClientSecretSection from './ClientSecretSection';
import useUpdateAgentCredentials from '../../../api/useUpdateAgentCredentials';
import {PERSONAL_AGENT_KIND, type Agent, type OAuthAgentConfig} from '../../../models/agent';

interface EditCredentialsSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  /** The credentials the agent type declares, such as a password. None, or a personal agent, shows no reset. */
  credentialFields?: CredentialResetField[];
}

export default function EditCredentialsSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  credentialFields = [],
}: EditCredentialsSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const updateCredentials = useUpdateAgentCredentials();

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

  // The server refuses an administrator's secret regeneration (AGT-1059) and credential reset (AGT-1069) for a
  // personal agent, whose credentials its owner manages, so neither is offered for one.
  const isPersonalAgent = agent.kind === PERSONAL_AGENT_KIND;

  return (
    <Stack spacing={3}>
      {oauth2Config && (
        <>
          <ClientIdSection oauth2Config={oauth2Config} />
          {!isPersonalAgent && (
            <ClientSecretSection agentId={agent.id} oauth2Config={oauth2Config} disabled={agent.isReadOnly} />
          )}
          <CertificateSection
            certificate={oauth2Config.certificate}
            onCertificateChange={(cert) => handleOAuth2ConfigChange({certificate: cert})}
            required={oauth2Config.tokenEndpointAuthMethod === 'private_key_jwt'}
            encryptionDependsOnCert={encryptionDependsOnCert}
            disabled={agent.isReadOnly}
          />
        </>
      )}
      {!isPersonalAgent && (
        <CredentialResetSection
          fields={credentialFields}
          disabled={agent.isReadOnly === true || !can('agent', 'update')}
          onReset={(field, newValue) =>
            updateCredentials.mutateAsync({agentId: agent.id, data: {credentials: {[field.fieldName]: newValue}}})
          }
          getErrorMessage={(error) =>
            getErrorMessage(
              error,
              // Resolves an error through the `agents` catalog, as the secret regeneration dialog does.
              (key, options) => t(key.includes(':') ? key : `agents:${key}`, options),
              'updateCredentials.error',
              'Failed to update credentials. Please try again.',
            )
          }
        />
      )}
    </Stack>
  );
}
