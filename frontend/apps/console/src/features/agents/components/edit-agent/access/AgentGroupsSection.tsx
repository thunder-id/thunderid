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
import RouteConfig from '../../../../../configs/RouteConfig';
import useGetAgentGroups from '../../../api/useGetAgentGroups';

interface AgentGroupsSectionProps {
  agentId: string;
}

export default function AgentGroupsSection({agentId}: AgentGroupsSectionProps): JSX.Element {
  const {t} = useTranslation();
  const {data, isLoading, isError} = useGetAgentGroups(agentId, {limit: 100, offset: 0});
  const groups = data?.groups ?? [];

  return (
    <SettingsCard
      title={t('agents:edit.access.groups.title', 'Groups')}
      description={
        <Trans
          i18nKey="agents:edit.access.groups.description"
          defaults="Groups this agent belongs to. Manage membership from the <manageLink>Groups page</manageLink>."
          components={{manageLink: <Link to={RouteConfig.groups.list()} />}}
        />
      }
    >
      {isLoading ? (
        <CircularProgress size={20} />
      ) : isError ? (
        <Alert severity="error">{t('agents:edit.access.groups.error', 'Failed to load groups for this agent.')}</Alert>
      ) : (
        <FormControl fullWidth>
          <FormLabel htmlFor="agent-groups">{t('agents:edit.access.groups.label', 'Groups')}</FormLabel>
          <Autocomplete
            multiple
            readOnly
            disableClearable
            forcePopupIcon={false}
            fullWidth
            options={[]}
            value={groups.map((group) => group.name)}
            renderInput={(params) => (
              <TextField
                {...params}
                id="agent-groups"
                placeholder={
                  groups.length === 0
                    ? t('agents:edit.access.groups.empty', 'This agent does not belong to any groups.')
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
