// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {
  IdentityAssertionsSection,
  type OAuth2Config,
  type OAuth2Token,
  applyGrantTypesChange,
  isIdJagEnabled,
} from '@thunderid/configure-applications';
import {Box, FormControlLabel, Stack, Switch, Typography} from '@wso2/oxygen-ui';
import {useEffect, useState} from 'react';
import {useTranslation} from 'react-i18next';
import AgentSignInSection from './AgentSignInSection';
import AllowedUserTypesSection from './AllowedUserTypesSection';
import DangerZoneSection from './DangerZoneSection';
import OperationModesSection from './OperationModesSection';
import OwnerSection from './OwnerSection';
import {DEFAULT_DELEGATED_GRANTS, DELEGATED_ONLY_GRANTS} from '../../../constants/delegationGrants';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';
import applyAgentPkceRule from '../../../utils/applyAgentPkceRule';
import {hasDelegationGrant} from '../../../utils/isDelegatedAgent';
import AgentDeleteDialog from '../../AgentDeleteDialog';

interface EditAdvancedSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onDeleteSuccess?: () => void;
  onBackchannelLogoutUriValidationChange?: (hasError: boolean) => void;
  /** Reports whether the Identity assertions (ID-JAG) card has a validation error, to gate Save. */
  onIdentityAssertionsValidationChange?: (hasError: boolean) => void;
}

export default function EditAdvancedSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onDeleteSuccess = undefined,
  onBackchannelLogoutUriValidationChange = undefined,
  onIdentityAssertionsValidationChange = undefined,
}: EditAdvancedSettingsProps) {
  const {t} = useTranslation();
  const [deleteDialogOpen, setDeleteDialogOpen] = useState(false);
  const isUnlocked = hasDelegationGrant(oauth2Config?.grantTypes);

  // The Identity assertions card does not report when it unmounts. Its only error that is not on a
  // staged value is a typed validity period it has not committed, which goes with this tab, so
  // leaving the tab clears the report. The edit page checks the staged values itself.
  useEffect(() => () => onIdentityAssertionsValidationChange?.(false), [onIdentityAssertionsValidationChange]);

  // Token and OAuth2 updates are merged in one change, so enabling identity assertions (which also
  // adds the token exchange grant) is applied against a single snapshot.
  const handleTokenConfigChange = (
    tokenUpdates: Partial<OAuth2Token>,
    oauth2Updates: Partial<OAuth2Config> = {},
  ): void => {
    const updates = applyAgentPkceRule(oauth2Updates);
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2'
        ? {
            ...auth,
            config: {...auth.config, ...updates, token: {...auth.config?.token, ...tokenUpdates}} as OAuthAgentConfig,
          }
        : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  const handleOAuth2ConfigChange = (updates: Partial<OAuth2Config>) => {
    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  // Delegated mode unlocks the delegated-only grants below and the Flows/Tokens tabs. Turning it on
  // gives the agent the default delegated grants and turning it off removes every delegated-only
  // grant; applyGrantTypesChange handles the dependent settings.
  const handleDelegationToggle = (checked: boolean): void => {
    if (!oauth2Config || checked === isUnlocked) return;
    const grantTypes = oauth2Config.grantTypes ?? [];
    const nextGrantTypes = checked
      ? [...new Set([...grantTypes, ...DEFAULT_DELEGATED_GRANTS])]
      : grantTypes.filter((grant) => !DELEGATED_ONLY_GRANTS.includes(grant));
    const updates = applyAgentPkceRule(applyGrantTypesChange(oauth2Config, nextGrantTypes));
    // Identity assertions are issued through token exchange, which turning Delegated mode off
    // removes, so they are turned off too. Their audiences and validity are kept for next time.
    if (!checked && isIdJagEnabled(oauth2Config)) {
      handleTokenConfigChange({idJag: {...oauth2Config.token?.idJag, enabled: false}}, updates);
      return;
    }
    handleOAuth2ConfigChange(updates);
  };

  return (
    <Stack spacing={3}>
      <SettingsCard title={t('agents:edit.advanced.mode.title', 'Operating mode')}>
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
              ? t('agents:edit.advanced.mode.onBehalfOfUser.description', 'This agent can act on behalf of a user.')
              : t(
                  'agents:edit.advanced.mode.onOwnBehalf.description',
                  'This agent authenticates with its own credentials without user interaction, using client credentials.',
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
        onBackchannelLogoutUriValidationChange={onBackchannelLogoutUriValidationChange}
        disabled={agent.isReadOnly}
      />
      {oauth2Config && isUnlocked && (
        <IdentityAssertionsSection
          oauth2Config={oauth2Config}
          onTokenConfigChange={handleTokenConfigChange}
          disabled={agent.isReadOnly}
          onValidationChange={onIdentityAssertionsValidationChange}
        />
      )}
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
