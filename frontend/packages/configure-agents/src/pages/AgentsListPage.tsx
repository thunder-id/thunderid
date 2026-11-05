// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentsList from '../components/AgentsList';
import useAgentRoutes from '../hooks/useAgentRoutes';

export default function AgentsListPage(): JSX.Element {
  const routes = useAgentRoutes();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentsListPage');

  return (
    <PageContent>
      <DocumentTitle title={t('agents:listing.title', 'Agents')} />
      <PageTitle>
        <PageTitle.Header>{t('agents:listing.title', 'Agents')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('agents:listing.subtitle', 'Manage service identities and machine clients')}{' '}
          <ExternalLink docKey="agents" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
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
            {t('agents:listing.addAgent', 'Add agent')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <AgentsList />
    </PageContent>
  );
}
