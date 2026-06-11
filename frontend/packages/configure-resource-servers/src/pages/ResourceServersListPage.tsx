/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle, Stack} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ResourceServersList from '../components/ResourceServersList';

export default function ResourceServersListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ResourceServersListPage');

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('resourceServers:listing.title', 'Resource Servers')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'resourceServers:listing.subtitle',
            'Define resource servers and their resources to manage access control.',
          )}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async (): Promise<void> => {
                  await navigate('/resource-servers/create');
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
