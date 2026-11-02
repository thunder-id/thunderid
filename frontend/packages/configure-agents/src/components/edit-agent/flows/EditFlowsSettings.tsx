// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  type Application,
  SettingsLockNotice,
  AuthenticationFlowSection,
  RegistrationFlowSection,
  hasUserAccess,
} from '@thunderid/configure-applications';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';
import {hasDelegationGrant} from '../../../utils/isDelegatedAgent';

interface EditFlowsSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function EditFlowsSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
}: EditFlowsSettingsProps): JSX.Element {
  const {t} = useTranslation();
  // Only the authorization code and CIBA grants run this agent's sign-in flows. Token exchange and
  // JWT bearer redeem an existing token or assertion, so they never run a sign-in flow.
  const isUnlocked = hasUserAccess(oauth2Config?.grantTypes);
  const isDelegated = hasDelegationGrant(oauth2Config?.grantTypes);

  // Agents share the inbound-client shape with applications (auth_flow_id, registration flow
  // config), so the same components are reused with an entity-label override. Forcing
  // isReadOnly disables every input via their existing disabled={application.isReadOnly} wiring
  // when the flows do not apply, without needing new props on the shared components.
  const appLikeAgent = {...agent, isReadOnly: (agent.isReadOnly ?? false) || !isUnlocked} as unknown as Application;
  const appLikeEditedAgent = editedAgent as unknown as Partial<Application>;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  return (
    <Stack spacing={3}>
      <SettingsLockNotice
        isUnlocked={isUnlocked}
        message={
          isDelegated
            ? t(
                'agents:edit.flows.signInGrantLock.message',
                'Sign-in flows apply only when this agent signs users in with the authorization code or CIBA grant type.',
              )
            : t(
                'agents:edit.flows.delegationLock.message',
                'These settings are frozen for this agent. Turn on Delegated mode in the Advanced tab to unlock and start using them.',
              )
        }
      >
        <Stack spacing={3}>
          <AuthenticationFlowSection
            application={appLikeAgent}
            editedApp={appLikeEditedAgent}
            onFieldChange={appHandleFieldChange}
            entityLabel="agent"
          />
          <RegistrationFlowSection
            application={appLikeAgent}
            editedApp={appLikeEditedAgent}
            onFieldChange={appHandleFieldChange}
            entityLabel="agent"
          />
        </Stack>
      </SettingsLockNotice>
    </Stack>
  );
}
