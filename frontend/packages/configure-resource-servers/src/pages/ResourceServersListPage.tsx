// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ResourceServersList from '../components/ResourceServersList';
import useResourceServerRoutes from '../hooks/useResourceServerRoutes';

export default function ResourceServersListPage(): JSX.Element {
  const navigate = useNavigate();
  const routes = useResourceServerRoutes();
  const {t} = useTranslation();
  const logger = useLogger('ResourceServersListPage');

  return (
    <PageContent>
      <DocumentTitle title={t('resourceServers:listing.title', 'Resource servers')} />
      <PageTitle>
        <PageTitle.Header>{t('resourceServers:listing.title', 'Resource servers')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'resourceServers:listing.subtitle',
            'Define resource servers and their resources to manage access control.',
          )}{' '}
          <ExternalLink docKey="resourceServers" />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async (): Promise<void> => {
                  await navigate(routes.create());
                })().catch((err: unknown) => {
                  logger.error('Failed to navigate to create resource server page', {error: err});
                });
              }}
            >
              {t('resourceServers:listing.addResourceServer', 'Add resource server')}
            </Button>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <ResourceServersList />
    </PageContent>
  );
}
