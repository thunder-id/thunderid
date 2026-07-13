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

import {SettingsCard} from '@thunderid/components';
import {Button, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {TokenEndpointAuthMethods} from '../../../../applications/models/oauth';
import type {OAuthAgentConfig} from '../../../models/agent';
import ClientSecretSuccessDialog from '../../ClientSecretSuccessDialog';
import RegenerateSecretDialog from '../../RegenerateSecretDialog';

interface ClientSecretSectionProps {
  agentId: string;
  oauth2Config?: OAuthAgentConfig;
  disabled?: boolean;
}

export default function ClientSecretSection({
  agentId,
  oauth2Config = undefined,
  disabled = false,
}: ClientSecretSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const [regenerateDialogOpen, setRegenerateDialogOpen] = useState(false);
  const [secretDialogOpen, setSecretDialogOpen] = useState(false);
  const [newClientSecret, setNewClientSecret] = useState('');

  const isConfidentialClient =
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_BASIC ||
    oauth2Config?.tokenEndpointAuthMethod === TokenEndpointAuthMethods.CLIENT_SECRET_POST;

  if (!isConfidentialClient) return null;

  return (
    <SettingsCard
      title={t('agents:edit.credentials.clientSecret.title', 'Client Secret')}
      description={t(
        'agents:edit.credentials.clientSecret.description',
        'The secret this agent uses to authenticate as a client.',
      )}
    >
      <Typography variant="body2" color="text.secondary" sx={{mb: 2}}>
        {t(
          'agents:edit.credentials.clientSecret.regenerateHint',
          'Client secret was shown once at creation. Regenerate to issue a new one.',
        )}
      </Typography>
      <Button variant="contained" color="error" onClick={() => setRegenerateDialogOpen(true)} disabled={disabled}>
        {t('agents:edit.credentials.clientSecret.regenerateButton', 'Regenerate secret')}
      </Button>

      <RegenerateSecretDialog
        open={regenerateDialogOpen}
        agentId={agentId}
        onClose={() => setRegenerateDialogOpen(false)}
        onSuccess={(clientSecret) => {
          setNewClientSecret(clientSecret);
          setSecretDialogOpen(true);
        }}
      />

      <ClientSecretSuccessDialog
        open={secretDialogOpen}
        clientSecret={newClientSecret}
        onClose={() => {
          setSecretDialogOpen(false);
          setNewClientSecret('');
        }}
      />
    </SettingsCard>
  );
}
