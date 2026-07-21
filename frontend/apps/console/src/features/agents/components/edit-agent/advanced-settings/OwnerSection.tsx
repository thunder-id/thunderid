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
import {useGetUsers} from '@thunderid/configure-users';
import {FormControl, FormLabel, MenuItem, Select, Typography} from '@wso2/oxygen-ui';
import {useMemo, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import type {Agent} from '../../../models/agent';

interface OwnerSectionProps {
  agent: Agent;
  editedAgent: Partial<Agent>;
  onFieldChange: (field: keyof Agent, value: unknown) => void;
}

interface UserOption {
  id: string;
  label: string;
}

const formatUserLabel = (user: {id: string; display?: string; attributes?: Record<string, unknown>}): string => {
  if (user.display) return user.display;
  const attrs = user.attributes ?? {};
  const username = typeof attrs.username === 'string' ? attrs.username : undefined;
  const email = typeof attrs.email === 'string' ? attrs.email : undefined;
  return username ?? email ?? user.id;
};

export default function OwnerSection({agent, editedAgent, onFieldChange}: OwnerSectionProps): JSX.Element {
  const {t} = useTranslation();

  const {data: usersData, isLoading: usersLoading} = useGetUsers({limit: 100, offset: 0});

  const options: UserOption[] = useMemo(
    () => (usersData?.users ?? []).map((user) => ({id: user.id, label: formatUserLabel(user)})),
    [usersData],
  );

  const ownerId = editedAgent.owner ?? agent.owner ?? '';

  return (
    <SettingsCard
      title={t('agents:edit.general.sections.owner.title', 'Owner')}
      description={t('agents:edit.general.sections.owner.description', 'The user accountable for this agent.')}
    >
      <FormControl fullWidth size="small">
        <FormLabel htmlFor="agent-owner-select">{t('agents:edit.general.sections.owner.label', 'Owner')}</FormLabel>
        <Select
          id="agent-owner-select"
          value={ownerId}
          displayEmpty
          disabled={usersLoading || agent.isReadOnly}
          onChange={(e) => onFieldChange('owner', e.target.value || undefined)}
          renderValue={(selected) =>
            !selected ? (
              <Typography color="text.secondary" variant="body2">
                {t('agents:edit.general.owner.empty', 'No owner assigned')}
              </Typography>
            ) : (
              (options.find((opt) => opt.id === selected)?.label ?? selected)
            )
          }
        >
          {options.map((opt) => (
            <MenuItem key={opt.id} value={opt.id}>
              {opt.label}
            </MenuItem>
          ))}
        </Select>
      </FormControl>
    </SettingsCard>
  );
}
