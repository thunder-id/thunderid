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

import {SettingsCard} from '@thunderid/components';
import {useGetUserTypes} from '@thunderid/configure-user-types';
import {Autocomplete, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {deriveOAuth2Flags} from '../../../../applications/utils/oauth2Rules';
import type {Agent, OAuthAgentConfig} from '../../../models/agent';

interface AllowedUserTypesSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  oauth2Config?: OAuthAgentConfig;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function AllowedUserTypesSection({
  agent,
  editedAgent,
  oauth2Config = undefined,
  onFieldChange,
}: AllowedUserTypesSectionProps): JSX.Element | null {
  const {t} = useTranslation();
  const {data: userTypesData, isLoading} = useGetUserTypes();

  // Allowed user types only matter when a person can actually sign in through this agent —
  // same dependency the redirect URI section has on authorization_code.
  const isApplicable = Boolean(oauth2Config && deriveOAuth2Flags(oauth2Config).hasAuthorizationCodeGrant);

  const value = editedAgent.allowedUserTypes ?? agent.allowedUserTypes ?? [];
  // The page-level Save guard computes this same check independently from state (see
  // AgentEditPage), since this section unmounts when its tab isn't active.
  const isMissingRequiredType = isApplicable && value.length === 0;

  if (!isApplicable) return null;

  const userTypeOptions = userTypesData?.types?.map((schema) => schema.name) ?? [];

  return (
    <SettingsCard
      title={t('agents:edit.flows.allowedUserTypes.title', 'Allowed User Types')}
      description={t(
        'agents:edit.flows.allowedUserTypes.description',
        'Restrict which user types can authenticate or register through this agent.',
      )}
    >
      <FormControl fullWidth>
        <FormLabel htmlFor="agent-allowed-user-types">
          {t('agents:edit.flows.allowedUserTypes.label', 'User Types')}
        </FormLabel>
        <Autocomplete
          multiple
          freeSolo
          fullWidth
          loading={isLoading}
          options={userTypeOptions}
          value={value}
          onChange={(_event, newValue) => onFieldChange('allowedUserTypes', newValue)}
          disabled={agent.isReadOnly}
          renderInput={(params) => (
            <TextField
              {...params}
              id="agent-allowed-user-types"
              placeholder={t('agents:edit.flows.allowedUserTypes.placeholder', 'Select or add user types')}
              error={isMissingRequiredType}
              helperText={
                isMissingRequiredType
                  ? t(
                      'agents:edit.flows.allowedUserTypes.required',
                      'Select at least one user type that can sign in through this agent.',
                    )
                  : t(
                      'agents:edit.flows.allowedUserTypes.hint',
                      'Only these user types can authenticate or register through this agent.',
                    )
              }
            />
          )}
        />
      </FormControl>
    </SettingsCard>
  );
}
