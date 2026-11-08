// Copyright 2025-2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {PageContent, PageTitle} from '@wso2/oxygen-ui';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import OrganizationUnitsTreeView from '../components/OrganizationUnitsTreeView';

/**
 * Props for the {@link OrganizationUnitsListPage} component.
 */
export interface OrganizationUnitsListPageProps {
  /**
   * Offer to create organization units only inside a project, on a deployment that places every organization
   * unit but the system one in a project.
   */
  createInProjectsOnly?: boolean;
}

export default function OrganizationUnitsListPage({
  createInProjectsOnly = false,
}: OrganizationUnitsListPageProps): JSX.Element {
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

      <OrganizationUnitsTreeView createInProjectsOnly={createInProjectsOnly} />
    </PageContent>
  );
}
