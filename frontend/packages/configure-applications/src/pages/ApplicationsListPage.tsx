// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import ApplicationsList from '../components/ApplicationsList';
import useApplicationRoutes from '../hooks/useApplicationRoutes';

export default function ApplicationsListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('ApplicationsListPage');
  const applicationRoutes = useApplicationRoutes();

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('applications:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('applications:listing.subtitle')} <ExternalLink docKey="applications" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            data-testid="application-add-button"
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              (async () => {
                await navigate(applicationRoutes.applications.types());
              })().catch((error: unknown) => {
                logger.error('Failed to navigate to create application page', {error});
              });
            }}
          >
            {t('applications:listing.addApplication')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <ApplicationsList />
    </PageContent>
  );
}
