// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  type Application,
  OAuth2GrantTypes,
  TokenAudienceSelector,
  type TokenAudienceOption,
  EditTokenSettings,
  hasUserAccess,
} from '@thunderid/configure-applications';
import {Alert, Link, Stack, Typography} from '@wso2/oxygen-ui';
import {Lock} from '@wso2/oxygen-ui-icons-react';
import {useEffect, useState, type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import AgentAccessTokenSection from './AgentAccessTokenSection';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';
import {hasDelegationGrant} from '../../../utils/isDelegatedAgent';

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

  const grantTypes = oauth2Config?.grantTypes ?? [];
  const isUnlocked = hasDelegationGrant(grantTypes);
  // Only authorization code and CIBA sign a user in through this agent, so only they issue ID and
  // refresh tokens and read attributes from the user's profile. hasUserAccess also counts the
  // password grant, which an agent cannot hold. Token exchange and JWT bearer issue an access token
  // alone: token exchange keeps the selected attributes the incoming token carries, and JWT bearer
  // adds none, since it does not resolve the assertion's subject to a local user.
  const isAccessTokenOnly = !hasUserAccess(grantTypes);
  const hasTokenExchange = grantTypes.includes(OAuth2GrantTypes.TOKEN_EXCHANGE);
  const hasJwtBearer = grantTypes.includes(OAuth2GrantTypes.JWT_BEARER);
  // A token exchange user comes from the incoming token and can be of any user type, while the
  // allowed user types field only appears with authorization code, so the attributes come from
  // every user type's schema.
  const listsAllUserTypes = isAccessTokenOnly && hasTokenExchange;

  // The user audience is not mounted without Delegated mode, so its last reported error state must
  // not keep blocking the save bar.
  useEffect(() => {
    onValidationChange?.((isUnlocked && userTabHasError) || agentTabHasError);
  }, [isUnlocked, userTabHasError, agentTabHasError, onValidationChange]);

  const appLikeAgent = {...agent, isReadOnly: agent.isReadOnly ?? false} as unknown as Application;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  // One line per grant, rather than translated sentences joined into one string.
  const accessTokenNotes: {key: string; text: string}[] = [];
  if (isAccessTokenOnly && hasTokenExchange) {
    accessTokenNotes.push({
      key: 'tokenExchange',
      text: t(
        'agents:edit.tokens.user.tokenExchangeAttributes.note',
        'Token exchange keeps only the selected attributes that the incoming token already carries.',
      ),
    });
  }
  if (isAccessTokenOnly && hasJwtBearer) {
    accessTokenNotes.push({
      key: 'jwtBearer',
      text: t(
        'agents:edit.tokens.user.jwtBearerAttributes.note',
        'Tokens from a redeemed ID-JAG carry no user attributes.',
      ),
    });
  }
  const accessTokenNote =
    accessTokenNotes.length > 0 ? (
      <>
        {accessTokenNotes.map((note) => (
          <Typography key={note.key} variant="body2">
            {note.text}
          </Typography>
        ))}
      </>
    ) : undefined;

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
            accessTokenOnly={isAccessTokenOnly}
            showAccessTokenAttributes={!isAccessTokenOnly || hasTokenExchange}
            accessTokenNote={accessTokenNote}
            userAttributeSource={listsAllUserTypes ? 'allUserTypes' : 'allowedUserTypes'}
          />
        </Stack>
      )}
    </TokenAudienceSelector>
  );
}
