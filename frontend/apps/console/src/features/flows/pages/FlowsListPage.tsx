/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import useGetFlows from '../api/useGetFlows';
import CapabilityCatalog from '../components/CapabilityCatalog';
import FlowsList from '../components/FlowsList';

export default function FlowsListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('FlowsListPage');
  const {data, isLoading, error} = useGetFlows();

  const hasFlows = (data?.flows?.length ?? 0) > 0;
  const isEmpty = !isLoading && !error && !hasFlows;

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('flows:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>{t('flows:listing.subtitle')}</PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              const handler = async () => {
                await navigate('/flows/create');
              };

              handler().catch((error: unknown) => {
                logger.error('Failed to navigate to flow builder page', {error});
              });
            }}
          >
            {t('flows:listing.addFlow')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      {isEmpty ? (
        <CapabilityCatalog variant="full" />
      ) : (
        <>
          {hasFlows && <CapabilityCatalog variant="compact" />}
          <FlowsList />
        </>
      )}
    </PageContent>
  );
}
