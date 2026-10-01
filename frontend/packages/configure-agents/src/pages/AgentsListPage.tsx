// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ExternalLink} from '@thunderid/components';
import {useGetAgentTypes} from '@thunderid/configure-agent-types';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {FileCog, Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentsList from '../components/AgentsList';
import useAgentRoutes from '../hooks/useAgentRoutes';
import {DEFAULT_AGENT_TYPE_NAME} from '../models/agent';

export default function AgentsListPage(): JSX.Element {
  const routes = useAgentRoutes();
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
      await navigate(routes.agentTypes.detail(defaultAgentType.id));
    })().catch((error: unknown) => {
      logger.error('Failed to navigate to agent type page', {error});
    });
  };

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('agents:listing.title', 'Agents')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('agents:listing.subtitle', 'Manage service identities and machine clients')}{' '}
          <ExternalLink docKey="agents" confirmBeforeNavigate={false} />
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
                await navigate(routes.agents.create());
              })().catch((error: unknown) => {
                logger.error('Failed to navigate to create agent page', {error});
              });
            }}
          >
            {t('agents:listing.addAgent', 'Add Agent')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <AgentsList />
    </PageContent>
  );
}
