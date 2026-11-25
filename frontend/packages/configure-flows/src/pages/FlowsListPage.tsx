// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import FlowsList from '../components/FlowsList';
import useFlowRoutes from '../hooks/useFlowRoutes';

/**
 * Props for the {@link FlowsListPage} component.
 */
export interface FlowsListPageProps {
  /**
   * Whether a flow is created in a project only. When set, a flow can be created inside a selected project and
   * not outside one.
   */
  createInProjectsOnly?: boolean;
}

export default function FlowsListPage({createInProjectsOnly = false}: FlowsListPageProps): JSX.Element {
  const navigate = useNavigate();
  const flowRoutes = useFlowRoutes();
  const {t} = useTranslation();
  const logger = useLogger('FlowsListPage');
  const {can} = useCapabilitiesContext();
  const {projectId} = useProjectScope();
  const canCreate = !createInProjectsOnly || Boolean(projectId);

  return (
    <PageContent>
      <DocumentTitle title={t('flows:listing.title', 'Flows')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('flows:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('flows:listing.subtitle')} <ExternalLink docKey="flows" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {canCreate && can('flow', 'create') && (
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                const handler = async () => {
                  await navigate(flowRoutes.flows.create());
                };

                handler().catch((error: unknown) => {
                  logger.error('Failed to navigate to flow builder page', {error});
                });
              }}
            >
              {t('flows:listing.addFlow')}
            </Button>
          )}
        </PageTitle.Actions>
      </PageTitle>

      <FlowsList />
    </PageContent>
  );
}
