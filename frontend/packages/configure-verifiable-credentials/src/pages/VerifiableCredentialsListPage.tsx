// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useCapabilitiesContext, useProjectScope} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {PageContent, PageTitle, Stack, Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import VerifiableCredentialsList, {type VerifiableCredentialsListProps} from '../components/VerifiableCredentialsList';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';

export interface VerifiableCredentialsListPageProps extends VerifiableCredentialsListProps {
  /**
   * Whether a credential configuration is created in a project only. When set, a configuration can be created
   * inside a selected project and not outside one.
   */
  createInProjectsOnly?: boolean;
}

export default function VerifiableCredentialsListPage({
  showWalletQr = false,
  createInProjectsOnly = false,
}: VerifiableCredentialsListPageProps): JSX.Element {
  const navigate = useNavigate();
  const {projectId} = useProjectScope();
  const canCreate = !createInProjectsOnly || Boolean(projectId);
  const {t} = useTranslation();
  const logger = useLogger('VerifiableCredentialsListPage');
  const routes = useVerifiableCredentialRoutes();
  const {can} = useCapabilitiesContext();

  return (
    <PageContent>
      <DocumentTitle title={t('verifiable-credentials:listing.title', 'Credential templates')} />
      <PageTitle>
        <PageTitle.Header>{t('verifiable-credentials:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('verifiable-credentials:listing.subtitle')}{' '}
          <ExternalLink docKey="verifiableCredentials.credentials" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        {canCreate && can('vc', 'create') && (
          <PageTitle.Actions>
            <Stack direction="row" spacing={2}>
              <Button
                data-testid="vc-add-button"
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
        )}
      </PageTitle>

      <VerifiableCredentialsList showWalletQr={showWalletQr} />
    </PageContent>
  );
}
