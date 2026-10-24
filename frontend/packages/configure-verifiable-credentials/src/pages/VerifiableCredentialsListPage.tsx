// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {PageContent, PageTitle, Stack, Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import VerifiableCredentialsList from '../components/VerifiableCredentialsList';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';

export default function VerifiableCredentialsListPage(): JSX.Element {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('VerifiableCredentialsListPage');
  const routes = useVerifiableCredentialRoutes();

  return (
    <PageContent>
      <DocumentTitle title={t('verifiable-credentials:listing.title', 'Credential templates')} />
      <PageTitle>
        <PageTitle.Header>{t('verifiable-credentials:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('verifiable-credentials:listing.subtitle')}{' '}
          <ExternalLink docKey="verifiableCredentials.credentials" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Stack direction="row" spacing={2}>
            <Button
              variant="contained"
              startIcon={<Plus size={18} />}
              onClick={() => {
                (async () => {
                  await navigate(routes.verifiableCredentials.create());
                })().catch((error: unknown) => {
                  logger.error('Failed to navigate to create page', {error});
                });
              }}
            >
              {t('verifiable-credentials:listing.add')}
            </Button>
          </Stack>
        </PageTitle.Actions>
      </PageTitle>

      <VerifiableCredentialsList />
    </PageContent>
  );
}
