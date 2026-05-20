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
import {Autocomplete, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import useGetUserTypes from '../../../../user-types/api/useGetUserTypes';
import type {Agent} from '../../../models/agent';

interface AllowedUserTypesSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

export default function AllowedUserTypesSection({
  agent,
  editedAgent,
  onFieldChange,
}: AllowedUserTypesSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data: userTypesData, isLoading} = useGetUserTypes();

  const userTypeOptions = userTypesData?.types?.map((schema) => schema.name) ?? [];
  const value = editedAgent.allowedUserTypes ?? agent.allowedUserTypes ?? [];

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
          renderInput={(params) => (
            <TextField
              {...params}
              id="agent-allowed-user-types"
              placeholder={t('agents:edit.flows.allowedUserTypes.placeholder', 'Select or add user types')}
              helperText={t('agents:edit.flows.allowedUserTypes.hint', 'Leave empty to allow any user type.')}
            />
          )}
        />
      </FormControl>
    </SettingsCard>
  );
}
