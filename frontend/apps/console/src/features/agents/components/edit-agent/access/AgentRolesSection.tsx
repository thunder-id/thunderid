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
import {Alert, Autocomplete, CircularProgress, FormControl, FormLabel, TextField} from '@wso2/oxygen-ui';
import {type JSX} from 'react';
import {Trans, useTranslation} from 'react-i18next';
import {Link} from 'react-router';
import useGetAgentRoles from '../../../api/useGetAgentRoles';

interface AgentRolesSectionProps {
  agentId: string;
}

export default function AgentRolesSection({agentId}: AgentRolesSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, isError} = useGetAgentRoles(agentId, {limit: 100, offset: 0});
  const roles = data?.roles ?? [];

  return (
    <SettingsCard
      title={t('agents:edit.access.roles.title', 'Roles')}
      description={
        <Trans
          i18nKey="agents:edit.access.roles.description"
          defaults="Roles assigned to this agent, directly or through its groups. Manage assignments from the <manageLink>Roles page</manageLink>."
          components={{manageLink: <Link to="/roles" />}}
        />
      }
    >
      {isLoading ? (
        <CircularProgress size={20} />
      ) : isError ? (
        <Alert severity="error">{t('agents:edit.access.roles.error', 'Failed to load roles for this agent.')}</Alert>
      ) : (
        <FormControl fullWidth>
          <FormLabel htmlFor="agent-roles">{t('agents:edit.access.roles.label', 'Roles')}</FormLabel>
          <Autocomplete
            multiple
            readOnly
            disableClearable
            forcePopupIcon={false}
            fullWidth
            options={[]}
            value={roles}
            renderInput={(params) => (
              <TextField
                {...params}
                id="agent-roles"
                placeholder={
                  roles.length === 0
                    ? t('agents:edit.access.roles.empty', 'This agent does not have any roles assigned.')
                    : undefined
                }
              />
            )}
          />
        </FormControl>
      )}
    </SettingsCard>
  );
}
