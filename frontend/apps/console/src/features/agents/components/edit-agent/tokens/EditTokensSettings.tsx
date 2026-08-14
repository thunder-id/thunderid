// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '@thunderid/configure-applications';
import type {Application} from '@thunderid/configure-applications';
import {Alert, Link, Stack} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import AgentAccessTokenSection from './AgentAccessTokenSection';
import TokenAudienceSelector, {
  type TokenAudienceOption,
} from '../../../../applications/components/common/TokenAudienceSelector';
import EditTokenSettings from '../../../../applications/components/edit-application/token-settings/EditTokenSettings';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

const AGENT_AUDIENCE = 'agent';
const USER_AUDIENCE = 'user';

interface EditTokensSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
  sectionResetKey?: number;
  /** Switches the edit page to its Advanced tab, where Delegated mode is turned on. */
  onNavigateToAdvanced?: () => void;
}

export default function EditTokensSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
  sectionResetKey = 0,
  onNavigateToAdvanced = undefined,
}: EditTokensSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [audience, setAudience] = useState(AGENT_AUDIENCE);
  const [userTabHasError, setUserTabHasError] = useState(false);
  const [agentTabHasError, setAgentTabHasError] = useState(false);

  const isUnlocked = oauth2Config?.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE) ?? false;

  // The user audience is not mounted without Delegated mode, so its last reported error state must
  // not keep blocking the save bar.
  useEffect(() => {
    onValidationChange?.((isUnlocked && userTabHasError) || agentTabHasError);
  }, [isUnlocked, userTabHasError, agentTabHasError, onValidationChange]);

  const appLikeAgent = {...agent, isReadOnly: agent.isReadOnly ?? false} as unknown as Application;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  const audienceOptions: TokenAudienceOption[] = [
    {
      value: AGENT_AUDIENCE,
      label: t('agents:edit.tokens.tabs.agent', 'Agent'),
      description: t('agents:edit.tokens.audience.agent.description', 'Tokens for the agent acting on its own'),
    },
    {
      value: USER_AUDIENCE,
      label: t('agents:edit.tokens.tabs.user', 'User'),
      description: t('agents:edit.tokens.audience.user.description', 'Tokens for the agent acting on behalf of a user'),
      isLocked: !isUnlocked,
    },
  ];

  return (
    <TokenAudienceSelector
      title={t('agents:edit.tokens.audience.title', 'Issued to')}
      options={audienceOptions}
      value={audience}
      onChange={setAudience}
      footnote={t(
        'agents:edit.tokens.audience.footnote',
        'Attribute sets are configured independently for each audience.',
      )}
    >
      {audience === AGENT_AUDIENCE && (
        <AgentAccessTokenSection
          key={sectionResetKey}
          agent={agent}
          editedAgent={editedAgent}
          oauth2Config={oauth2Config}
          onFieldChange={onFieldChange}
          onValidationChange={setAgentTabHasError}
        />
      )}
      {audience === USER_AUDIENCE && !isUnlocked && (
        <Alert severity="info" icon={<Lock size={20} />}>
          <Trans
            i18nKey="agents:edit.tokens.delegationLock.message"
            defaults="This agent does not receive tokens on behalf of a user. Turn on Delegated mode in the <advancedLink>Advanced tab</advancedLink> to configure them."
            components={{
              advancedLink: <Link component="button" type="button" underline="always" onClick={onNavigateToAdvanced} />,
            }}
          />
        </Alert>
      )}
      {audience === USER_AUDIENCE && isUnlocked && (
        <Stack spacing={3}>
          <EditTokenSettings
            application={appLikeAgent}
            oauth2Config={oauth2Config}
            onFieldChange={appHandleFieldChange}
            onValidationChange={setUserTabHasError}
            entityLabel="agent"
            showUserInfoTab={false}
            showActorClaim
            actorSub={agent.id}
            certificateLocation="Credentials"
            sectionResetKey={sectionResetKey}
          />
        </Stack>
      )}
    </TokenAudienceSelector>
  );
}
