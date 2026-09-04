// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {OAuth2GrantTypes} from '@thunderid/configure-applications';
import type {OAuth2Config} from '@thunderid/configure-applications';
import {Box, FormControlLabel, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {useState} from 'react';
import {useTranslation} from 'react-i18next';
import AgentSignInSection from './AgentSignInSection';
import AllowedUserTypesSection from './AllowedUserTypesSection';
import DangerZoneSection from './DangerZoneSection';
import OperationModesSection from './OperationModesSection';
import OwnerSection from './OwnerSection';
import {applyGrantTypesChange} from '../../../../applications/utils/oauth2Rules';
import {DELEGATED_ONLY_GRANTS} from '../../../constants/delegationGrants';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';
import AgentDeleteDialog from '../../AgentDeleteDialog';

interface EditAdvancedSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onDeleteSuccess?: () => void;
}

export default function EditAdvancedSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onDeleteSuccess = undefined,
}: EditAdvancedSettingsProps) {
  const {t} = useTranslation();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const isUnlocked = oauth2Config?.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE) ?? false;

  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  // Delegated mode unlocks the delegated-only grants below and the Flows/Tokens tabs. Toggling it
  // just flips authorization_code on/off; applyGrantTypesChange handles the dependent grants.
  const handleDelegationToggle = (checked: boolean): void => {
    if (!oauth2Config || checked === isUnlocked) return;
    const grantTypes = oauth2Config.grantTypes ?? [];
    const nextGrantTypes = checked
      ? [...new Set([...grantTypes, OAuth2GrantTypes.AUTHORIZATION_CODE])]
      : grantTypes.filter((grant) => !DELEGATED_ONLY_GRANTS.includes(grant));
    const updates = applyGrantTypesChange(oauth2Config, nextGrantTypes);
    // PKCE is fully derived from authorization_code for agents.
    if (checked) {
      updates.pkceRequired = true;
    }
    handleOAuth2ConfigChange(updates);
  };

  return (
    <Stack spacing={3}>
      <SettingsCard title={t('agents:edit.advanced.mode.title', 'Operating Mode')}>
        <Box>
          <FormControlLabel
            control={
              <Switch
                checked={isUnlocked}
                onChange={(e) => handleDelegationToggle(e.target.checked)}
                disabled={!oauth2Config || agent.isReadOnly === true}
              />
            }
            label={
              <Typography variant="subtitle2">
                {t('agents:edit.advanced.delegationToggle.label', 'Delegated mode')}
              </Typography>
            }
          />
          <Typography variant="caption" color="text.secondary" sx={{display: 'block', ml: '52px'}}>
            {isUnlocked
              ? t(
                  'agents:edit.advanced.mode.onBehalfOfUser.description',
                  'This agent acts on behalf of a signed-in user, using Authorization Code with PKCE.',
                )
              : t(
                  'agents:edit.advanced.mode.onOwnBehalf.description',
                  'This agent authenticates with its own credentials without user interaction, using Client Credentials.',
                )}
          </Typography>
        </Box>
      </SettingsCard>

      <OwnerSection agent={agent} editedAgent={editedAgent} onFieldChange={onFieldChange} />
      <AllowedUserTypesSection
        agent={agent}
        editedAgent={editedAgent}
        oauth2Config={oauth2Config}
        onFieldChange={onFieldChange}
      />
      <AgentSignInSection
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
      {!agent.isReadOnly && <DangerZoneSection onDeleteClick={() => setDeleteDialogOpen(true)} />}

      <AgentDeleteDialog
        open={deleteDialogOpen}
        agentId={agent.id}
        onClose={() => setDeleteDialogOpen(false)}
        onSuccess={onDeleteSuccess}
      />
    </Stack>
  );
}
