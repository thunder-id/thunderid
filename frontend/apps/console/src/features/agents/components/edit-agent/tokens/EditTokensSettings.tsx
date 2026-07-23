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

import {Box, Stack, Tab, Tabs} from '@wso2/oxygen-ui';
import {useEffect, useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import AgentAccessTokenSection from './AgentAccessTokenSection';
import EditTokenSettings from '../../../../applications/components/edit-application/token-settings/EditTokenSettings';
import type {Application} from '../../../../applications/models/application';
import {OAuth2GrantTypes} from '../../../../applications/models/oauth';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';
import DelegationLockNotice from '../shared/DelegationLockNotice';

interface EditTokensSettingsProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
  onValidationChange?: (hasErrors: boolean) => void;
}

export default function EditTokensSettings({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
  onValidationChange = undefined,
}: EditTokensSettingsProps): JSX.Element {
  const {t} = useTranslation();
  const [subTab, setSubTab] = useState(0);
  const [userTabHasError, setUserTabHasError] = useState(false);
  const [agentTabHasError, setAgentTabHasError] = useState(false);

  const isUnlocked = oauth2Config?.grantTypes?.includes(OAuth2GrantTypes.AUTHORIZATION_CODE) ?? false;

  useEffect(() => {
    onValidationChange?.(userTabHasError || agentTabHasError);
  }, [userTabHasError, agentTabHasError, onValidationChange]);

  // Forcing isReadOnly disables every input via EditTokenSettings' existing
  // disabled={application.isReadOnly} wiring when Delegated mode isn't on.
  const appLikeAgent = {...agent, isReadOnly: (agent.isReadOnly ?? false) || !isUnlocked} as unknown as Application;
  const appHandleFieldChange = onFieldChange as unknown as (field: keyof Application, value: unknown) => void;

  const handleSubTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setSubTab(newValue);
  };

  return (
    <Box>
      <Tabs value={subTab} onChange={handleSubTabChange} aria-label="agent token settings sub-tabs">
        <Tab label={t('agents:edit.tokens.tabs.agent', 'Agent')} sx={{textTransform: 'none'}} />
        <Tab label={t('agents:edit.tokens.tabs.user', 'User')} sx={{textTransform: 'none'}} />
      </Tabs>
      <Box sx={{pt: 3}}>
        {subTab === 0 && (
          <AgentAccessTokenSection
            agent={agent}
            editedAgent={editedAgent}
            oauth2Config={oauth2Config}
            onFieldChange={onFieldChange}
            onValidationChange={setAgentTabHasError}
          />
        )}
        {subTab === 1 && (
          <DelegationLockNotice
            isUnlocked={isUnlocked}
            message={t(
              'agents:edit.tokens.delegationLock.message',
              'These settings are frozen for this agent. Turn on Delegated mode in the Flows tab to unlock and start using them.',
            )}
          >
            <Stack spacing={3}>
              <EditTokenSettings
                application={appLikeAgent}
                oauth2Config={oauth2Config}
                onFieldChange={appHandleFieldChange}
                onValidationChange={setUserTabHasError}
                entityLabel="agent"
                showUserInfoTab={false}
                showActorClaim
              />
            </Stack>
          </DelegationLockNotice>
        )}
      </Box>
    </Box>
  );
}
