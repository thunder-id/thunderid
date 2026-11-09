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
import ApplicationsList from '../components/ApplicationsList';
import useApplicationRoutes from '../hooks/useApplicationRoutes';

/**
 * Props for the {@link ApplicationsListPage} component.
 */
export interface ApplicationsListPageProps {
  /**
   * Whether an application is created in a project only. When set, an application can be created inside a selected project and
   * not outside one.
   */
  createInProjectsOnly?: boolean;
}

export default function ApplicationsListPage({createInProjectsOnly = false}: ApplicationsListPageProps): JSX.Element {
  const routes = useApplicationRoutes();
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ApplicationsListPage');
  const {can} = useCapabilitiesContext();
  const {projectId} = useProjectScope();
  const canCreate = !createInProjectsOnly || Boolean(projectId);

  return (
    <PageContent>
      <DocumentTitle title={t('applications:listing.title', 'Applications')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('applications:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('applications:listing.subtitle')} <ExternalLink docKey="applications" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          {canCreate && can('app', 'create') && (
            <Button
              data-testid="application-add-button"
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async () => {
                  await navigate(routes.applications.types());
                })().catch((error: unknown) => {
                  logger.error('Failed to navigate to create application page', {error});
                });
              }}
            >
              {t('applications:listing.addApplication')}
            </Button>
          )}
        </PageTitle.Actions>
      </PageTitle>

      <ApplicationsList />
    </PageContent>
  );
}
