// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {PageContent, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import OrganizationUnitsTreeView from '../components/OrganizationUnitsTreeView';

export default function OrganizationUnitsListPage(): JSX.Element {
  const {t} = useTranslation();

  return (
    <PageContent>
      <DocumentTitle title={t('organizationUnits:listing.title', 'Organization units (OU)')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('organizationUnits:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('organizationUnits:listing.subtitle')} <ExternalLink docKey="organizationUnits" />
        </PageTitle.SubHeader>
      </PageTitle>

      <OrganizationUnitsTreeView />
    </PageContent>
  );
}
