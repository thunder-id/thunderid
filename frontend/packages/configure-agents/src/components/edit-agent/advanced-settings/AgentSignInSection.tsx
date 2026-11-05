// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {ApplicationConstants, deriveOAuth2Flags} from '@thunderid/configure-applications';
import {Switch, Typography} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

interface AgentSignInSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function AgentSignInSection({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
}: AgentSignInSectionProps): JSX.Element | null {
  const {t} = useTranslation();

  // Agent sign-in only matters when an agent can actually sign in through this agent — the same
  // dependency on authorization_code that the allowed user types section has.
  const isApplicable = Boolean(oauth2Config && deriveOAuth2Flags(oauth2Config).hasAuthorizationCodeGrant);

  if (!isApplicable) return null;

  // Agent access is a single on/off choice in the console: on allows every agent type, off allows
  // none. Any non-empty list, such as an older `['default']`, shows as on, and a list that names agent
  // types is spelled out, since turning the switch back on allows every agent type instead.
  const allowedAgentTypes = editedAgent.allowedAgentTypes ?? agent.allowedAgentTypes ?? [];
  const isEnabled = allowedAgentTypes.length > 0;
  const restrictedAgentTypes = allowedAgentTypes.includes(ApplicationConstants.ALL_AGENT_TYPES)
    ? []
    : allowedAgentTypes;

  const handleToggle = (enabled: boolean): void => {
    onFieldChange('allowedAgentTypes', enabled ? [ApplicationConstants.ALL_AGENT_TYPES] : []);
  };

  const toggleLabel = t('agents:edit.advanced.agentSignIn.toggle.label', 'Enable agent sign-in');

  return (
    <SettingsCard
      title={t('agents:edit.advanced.agentSignIn.title', 'Agent sign-in')}
      description={t(
        'agents:edit.advanced.agentSignIn.description',
        'Allow agents of any agent type to sign in through this agent using the sign-in flow.',
      )}
      headerAction={
        <Switch
          checked={isEnabled}
          onChange={(event) => handleToggle(event.target.checked)}
          disabled={agent.isReadOnly}
          slotProps={{input: {'aria-label': toggleLabel}}}
        />
      }
    >
      {restrictedAgentTypes.length > 0 ? (
        <Typography variant="body2" color="text.secondary">
          {t(
            'agents:edit.advanced.agentSignIn.restricted',
            'Only agents of these agent types can sign in now: {{types}}. Turning agent sign-in off and on again allows every agent type.',
            {types: restrictedAgentTypes.join(', ')},
          )}
        </Typography>
      ) : null}
    </SettingsCard>
  );
}
