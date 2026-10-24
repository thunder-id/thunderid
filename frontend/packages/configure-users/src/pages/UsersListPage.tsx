// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import UsersList from '../components/UsersList';
import useUserRoutes from '../hooks/useUserRoutes';

export default function UsersListPage() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UsersListPage');
  const routes = useUserRoutes();

  return (
    <PageContent>
      <DocumentTitle title={t('users:title', 'Users')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('users:title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('users:subtitle')} <ExternalLink docKey="users" />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={20} />}
            onClick={() => {
              (async () => {
                await navigate(routes.add());
              })().catch((error: unknown) => {
                logger.error('Failed to navigate to add user page', {error});
              });
            }}
          >
            {t('users:addUser')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <UsersList />
    </PageContent>
  );
}
