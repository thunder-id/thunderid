/**
 * Copyright (c) 2025-2026, WSO2 LLC. (https://www.wso2.com).
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

import {LearnMoreLink} from '@thunderid/components';
import {PageContent, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import OrganizationUnitsTreeView from '../components/OrganizationUnitsTreeView';

export default function OrganizationUnitsListPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('organizationUnits:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('organizationUnits:listing.subtitle')} <LearnMoreLink docKey="organizationUnits" />
        </PageTitle.SubHeader>
      </PageTitle>

      <OrganizationUnitsTreeView />
    </PageContent>
  );
}
