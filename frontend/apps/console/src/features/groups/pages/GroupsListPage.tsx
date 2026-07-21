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
import {Stack, Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import GroupsList from '../components/GroupsList';

export default function GroupsListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('GroupsListPage');

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('groups:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>{t('groups:listing.subtitle')}</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async () => {
                  await navigate('/groups/create');
                })().catch((error: unknown) => {
                  logger.error('Failed to navigate to create group page', {error});
                });
              }}
            >
              {t('groups:listing.addGroup')}
            </Button>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <GroupsList />
    </PageContent>
  );
}
