// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import AuthorizationDetailTypesList from '../components/authorization-detail-types/AuthorizationDetailTypesList';
import useAuthorizationDetailTypeRoutes from '../hooks/useAuthorizationDetailTypeRoutes';

/**
 * Props for the {@link AuthorizationDetailTypesListPage} component.
 */
export interface AuthorizationDetailTypesListPageProps {
  /**
   * Whether an intent is created in a project only. When set, an intent can be created inside a selected project
   * and not outside one.
   */
  createInProjectsOnly?: boolean;
}

export default function AuthorizationDetailTypesListPage({
  createInProjectsOnly = false,
}: AuthorizationDetailTypesListPageProps): JSX.Element {
  const navigate = useNavigate();
  const routes = useAuthorizationDetailTypeRoutes();
  const {t} = useTranslation();
  const {can} = useCapabilitiesContext();
  const logger = useLogger('AuthorizationDetailTypesListPage');
  const {projectId} = useProjectScope();
  const canCreate = can('authzdetail', 'create') && (!createInProjectsOnly || Boolean(projectId));

  return (
    <PageContent>
      <DocumentTitle title={t('authorizationDetailTypes:listing.title', 'Intents')} />
      <PageTitle>
        <PageTitle.Header>{t('authorizationDetailTypes:listing.title', 'Intents')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'authorizationDetailTypes:listing.subtitle',
            'Define fine-grained, structured permissions that clients can ask users to approve through Rich Authorization Requests (RAR).',
          )}{' '}
          <ExternalLink docKey="authorizationDetailTypes" />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            {canCreate && (
              <Button
                variant="contained"
                startIcon={<Plus size={18} />}
                onClick={() => {
                  (async (): Promise<void> => {
                    await navigate(routes.create());
                  })().catch((err: unknown) => {
                    logger.error('Failed to navigate to create intent page', {error: err});
                  });
                }}
              >
                {t('authorizationDetailTypes:listing.addIntent', 'Add intent')}
              </Button>
            )}
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <AuthorizationDetailTypesList />
    </PageContent>
  );
}
