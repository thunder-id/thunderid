// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX, ReactNode} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import UsersList from '../components/UsersList';
import useUserRoutes from '../hooks/useUserRoutes';

export interface UsersListPageProps {
  /** The page's title, "Users" otherwise, for a console whose users are of one kind, such as administrators. */
  title?: string;
  /** Shown above the list, such as what the users are for. */
  notice?: ReactNode;
}

export default function UsersListPage({title = undefined, notice = undefined}: UsersListPageProps = {}): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UsersListPage');
  const routes = useUserRoutes();

  return (
    <PageContent>
      <DocumentTitle title={title ?? t('users:title', 'Users')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{title ?? t('users:title')}</PageTitle.Header>
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

      {notice}
      <UsersList />
    </PageContent>
  );
}
