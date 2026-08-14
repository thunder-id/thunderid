// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {OAuth2GrantTypes} from '@thunderid/configure-applications';
import type {Application} from '@thunderid/configure-applications';
import {Stack} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import SettingsLockNotice from '../../../../applications/components/common/SettingsLockNotice';
import AuthenticationFlowSection from '../../../../applications/components/edit-application/flows-settings/AuthenticationFlowSection';
import RegistrationFlowSection from '../../../../applications/components/edit-application/flows-settings/RegistrationFlowSection';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

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
  const isUnlocked = oauth2Config?.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE) ?? false;

  // Agents share the inbound-client shape with applications (auth_flow_id, registration flow
  // config), so the same components are reused with an entity-label override. Forcing
  // isReadOnly disables every input via their existing disabled={application.isReadOnly} wiring
  // when Delegated mode isn't on, without needing new props on the shared components.
  const appLikeAgent = {...agent, isReadOnly: (agent.isReadOnly ?? false) || !isUnlocked} as unknown as Application;
  const appLikeEditedAgent = editedAgent as unknown as Partial<Application>;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  return (
    <Stack spacing={3}>
      <SettingsLockNotice
        isUnlocked={isUnlocked}
        message={t(
          'agents:edit.flows.delegationLock.message',
          'These settings are frozen for this agent. Turn on Delegated mode in the Advanced tab to unlock and start using them.',
        )}
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
