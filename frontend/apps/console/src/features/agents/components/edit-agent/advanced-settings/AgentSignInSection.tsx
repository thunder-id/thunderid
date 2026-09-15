// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {SettingsCard} from '@thunderid/components';
import {Switch} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {deriveOAuth2Flags} from '../../../../applications/utils/oauth2Rules';
import {DEFAULT_AGENT_TYPE_NAME, type Agent, type OAuthAgentConfig} from '../../../models/agent';

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

  // Agent access is a single on/off choice in the console for now: on means the default agent type
  // (the only type) is the only allowed one, off means no agent type is allowed.
  const isEnabled = (editedAgent.allowedAgentTypes ?? agent.allowedAgentTypes ?? []).length > 0;

  const handleToggle = (enabled: boolean): void => {
    onFieldChange('allowedAgentTypes', enabled ? [DEFAULT_AGENT_TYPE_NAME] : []);
  };

  const toggleLabel = t('agents:edit.advanced.agentSignIn.toggle.label', 'Enable Agent Sign-In');

  return (
    <SettingsCard
      title={t('agents:edit.advanced.agentSignIn.title', 'Agent Sign-In')}
      description={t(
        'agents:edit.advanced.agentSignIn.description',
        'Allow agents to sign in through this agent using the sign-in flow.',
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
      {null}
    </SettingsCard>
  );
}
