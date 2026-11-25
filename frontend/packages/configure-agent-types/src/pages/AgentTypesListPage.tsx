// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useCapabilitiesContext} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AgentTypesList from '../components/AgentTypesList';
import useAgentTypeRoutes from '../hooks/useAgentTypeRoutes';

/**
 * Lists every agent type, with an action to create one.
 *
 * @public
 */
export default function AgentTypesListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('AgentTypesListPage');
  const routes = useAgentTypeRoutes();
  const {can} = useCapabilitiesContext();

  return (
    <PageContent>
      <DocumentTitle title={t('agentTypes:title', 'Agent types')} />
      <PageTitle>
        <PageTitle.Header>{t('agentTypes:title', 'Agent types')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('agentTypes:subtitle', 'Define and manage agent types with custom schemas')}{' '}
          <ExternalLink docKey="agentTypes" />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {can('agenttype', 'create') && (
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async () => {
                  await navigate(routes.create());
                })().catch((error: unknown) => {
                  logger.error('Failed to navigate to create agent type page', {error});
                });
              }}
            >
              {t('agentTypes:createAgentType', 'Create agent type')}
            </Button>
          )}
        </PageTitle.Actions>
      </PageTitle>

      <AgentTypesList />
    </PageContent>
  );
}
