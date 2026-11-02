// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Stack, Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import RolesList from '../components/RolesList';
import useRoleRoutes from '../hooks/useRoleRoutes';

export default function RolesListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('RolesListPage');
  const routes = useRoleRoutes();

  return (
    <PageContent>
      <DocumentTitle title={t('roles:listing.title', 'Roles')} />
      <PageTitle>
        <PageTitle.Header>{t('roles:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('roles:listing.subtitle')} <ExternalLink docKey="roles" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async () => {
                  await navigate(routes.roles.create());
                })().catch((error: unknown) => {
                  logger.error('Failed to navigate to create role page', {error});
                });
              }}
            >
              {t('roles:listing.addRole')}
            </Button>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <RolesList />
    </PageContent>
  );
}
