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

import {useGetAgentTypes} from '@thunderid/configure-agent-types';
import {useLogger} from '@thunderid/logger/react';
import {Stack, Button, TextField, InputAdornment, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {FileCog, Plus, Search} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentsList from '../components/AgentsList';
import {DEFAULT_AGENT_TYPE_NAME} from '../models/agent';

export default function AgentsListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsListPage');

  // Agent types are restricted to a single bootstrap-provisioned `default` schema; the Schema
  // button jumps to its edit page so operators can manage attribute definitions in place.
  const {data: agentTypesData, isLoading: isAgentTypesLoading} = useGetAgentTypes();
  const defaultAgentType = agentTypesData?.types?.find((s) => s.name === DEFAULT_AGENT_TYPE_NAME);

  const handleSchemaClick = (): void => {
    if (!defaultAgentType) return;
    (async () => {
      await navigate(`/agent-types/${defaultAgentType.id}`);
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to agent type page', {error});
    });
  };

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('agents:listing.title', 'Agents')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('agents:listing.subtitle', 'Manage service identities and machine clients')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            data-testid="agent-schema-button"
            variant="outlined"
            startIcon={<FileCog size={18} />}
            disabled={isAgentTypesLoading || !defaultAgentType}
            onClick={handleSchemaClick}
          >
            {t('agents:listing.schema', 'Schema')}
          </Button>
          <Button
            data-testid="agent-add-button"
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              (async () => {
                await navigate('/agents/create');
              })().catch((error: unknown) => {
                logger.error('Failed to navigate to create agent page', {error});
              });
            }}
          >
            {t('agents:listing.addAgent', 'Add Agent')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <Stack direction="row" spacing={2} mb={4} flexWrap="wrap" useFlexGap>
        <TextField
          placeholder={t('agents:listing.search.placeholder', 'Search agents')}
          size="small"
          sx={{flexGrow: 1, minWidth: 300}}
          slotProps={{
            input: {
              startAdornment: (
                <InputAdornment position="start">
                  <Search size={16} />
                </InputAdornment>
              ),
            },
          }}
        />
      </Stack>

      <AgentsList />
    </PageContent>
  );
}
