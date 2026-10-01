// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import type {JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import FlowsList from '../components/FlowsList';
import useFlowRoutes from '../hooks/useFlowRoutes';

export default function FlowsListPage(): JSX.Element {
  const navigate = useNavigate();
  const flowRoutes = useFlowRoutes();
  const {t} = useTranslation();
  const logger = useLogger('FlowsListPage');

  return (
    <PageContent>
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('flows:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('flows:listing.subtitle')} <ExternalLink docKey="flows" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              const handler = async () => {
                await navigate(flowRoutes.flows.create());
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

      <FlowsList />
    </PageContent>
  );
}
