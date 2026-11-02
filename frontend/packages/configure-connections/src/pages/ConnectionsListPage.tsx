// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {PageContent, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import ConnectionsList from '../components/ConnectionsList';

export default function ConnectionsListPage(): JSX.Element {
  const {t} = useTranslation('connections');

  return (
    <PageContent>
      <DocumentTitle title={t('listing.title', 'Connections')} />
      <PageTitle>
        <PageTitle.Header>{t('listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('listing.subtitle')} <ExternalLink docKey="connections" />
        </PageTitle.SubHeader>
      </PageTitle>

      <ConnectionsList />
    </PageContent>
  );
}
