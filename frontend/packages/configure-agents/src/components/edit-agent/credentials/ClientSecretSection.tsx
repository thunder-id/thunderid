// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {TokenEndpointAuthMethods} from '@thunderid/configure-applications';
import {Button, FormControl, FormLabel, Stack, TextField, Typography} from '@wso2/oxygen-ui';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
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
      title={t('agents:edit.credentials.sections.secret.title', 'Secret')}
      description={t(
        'agents:edit.credentials.sections.secret.description',
        'Regenerating the secret immediately invalidates the current one and cannot be undone.',
      )}
    >
      <FormControl fullWidth>
        <FormLabel htmlFor="agent-credentials-secret">
          {t('agents:edit.credentials.sections.secret.clientSecretLabel', 'Client Secret')}
        </FormLabel>
        <Typography variant="caption" color="text.secondary" sx={{display: 'block', mb: 1}}>
          {t(
            'agents:edit.credentials.sections.secret.hint',
            'A confidential credential used with the Client ID to authenticate this agent. Keep it secret.',
          )}
        </Typography>
        <Stack direction="row" spacing={1}>
          <TextField
            fullWidth
            id="agent-credentials-secret"
            value="••••••••••••••••"
            InputProps={{readOnly: true}}
            disabled
            sx={{flex: '0 0 80%', '& input': {fontFamily: 'monospace', fontSize: '0.875rem'}}}
          />
          <Button
            variant="contained"
            color="error"
            onClick={() => setRegenerateDialogOpen(true)}
            disabled={disabled}
            sx={{flex: '0 0 20%'}}
          >
            {t('agents:edit.credentials.sections.secret.regenerateButton', 'Regenerate Client Secret')}
          </Button>
        </Stack>
      </FormControl>

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
