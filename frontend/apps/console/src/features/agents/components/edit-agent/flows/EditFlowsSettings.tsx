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

import {FormControlLabel, Stack, Switch} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import AuthenticationFlowSection from '../../../../applications/components/edit-application/flows-settings/AuthenticationFlowSection';
import RecoveryFlowSection from '../../../../applications/components/edit-application/flows-settings/RecoveryFlowSection';
import RegistrationFlowSection from '../../../../applications/components/edit-application/flows-settings/RegistrationFlowSection';
import type {Application} from '../../../../applications/models/application';
import {OAuth2GrantTypes} from '../../../../applications/models/oauth';
import {applyGrantTypesChange} from '../../../../applications/utils/oauth2Rules';
import {DELEGATED_ONLY_GRANTS} from '../../../constants/delegationGrants';
import type {Agent, AgentInboundAuthConfig, OAuthAgentConfig} from '../../../models/agent';
import DelegationLockNotice from '../shared/DelegationLockNotice';

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

  // Lets the user turn on Delegated mode directly from this tab instead of having to visit
  // Advanced — same grant-type rules as OperationModesSection's mode toggle.
  const handleDelegationToggle = (checked: boolean): void => {
    if (!oauth2Config || checked === isUnlocked) return;
    const grantTypes = oauth2Config.grantTypes ?? [];
    const nextGrantTypes = checked
      ? [...new Set([...grantTypes, OAuth2GrantTypes.AUTHORIZATION_CODE])]
      : grantTypes.filter((grant) => !DELEGATED_ONLY_GRANTS.includes(grant));
    const updates = applyGrantTypesChange(oauth2Config, nextGrantTypes);
    // PKCE is fully derived from authorization_code for agents (mirrors OperationModesSection).
    if (checked) {
      updates.pkceRequired = true;
    }

    const currentInboundAuth: AgentInboundAuthConfig[] = editedAgent.inboundAuthConfig ?? agent.inboundAuthConfig ?? [];
    const updatedInboundAuth = currentInboundAuth.map((auth) =>
      auth.type === 'oauth2' ? {...auth, config: {...auth.config, ...updates} as OAuthAgentConfig} : auth,
    );
    onFieldChange('inboundAuthConfig', updatedInboundAuth);
  };

  // Agents share the inbound-client shape with applications (auth_flow_id, registration/recovery
  // flow config), so the same components are reused with an entity-label override. Forcing
  // isReadOnly disables every input via their existing disabled={application.isReadOnly} wiring
  // when Delegated mode isn't on, without needing new props on the shared components.
  const appLikeAgent = {...agent, isReadOnly: (agent.isReadOnly ?? false) || !isUnlocked} as unknown as Application;
  const appLikeEditedAgent = editedAgent as unknown as Partial<Application>;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  return (
    <Stack spacing={3}>
      <FormControlLabel
        control={
          <Switch
            checked={isUnlocked}
            onChange={(e) => handleDelegationToggle(e.target.checked)}
            disabled={!oauth2Config || agent.isReadOnly === true}
          />
        }
        label={t('agents:edit.flows.delegationToggle.label', 'Delegated mode')}
      />
      <DelegationLockNotice
        isUnlocked={isUnlocked}
        message={t(
          'agents:edit.flows.delegationLock.message',
          'These settings are frozen for this agent. Turn on Delegated mode above to unlock and start using them.',
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
          <RecoveryFlowSection
            application={appLikeAgent}
            editedApp={appLikeEditedAgent}
            onFieldChange={appHandleFieldChange}
            entityLabel="agent"
          />
        </Stack>
      </DelegationLockNotice>
    </Stack>
  );
}
