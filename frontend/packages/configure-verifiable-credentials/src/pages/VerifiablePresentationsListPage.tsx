// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useProjectScope} from '@thunderid/contexts';
import {useLogger} from '@thunderid/logger/react';
import {PageContent, PageTitle, Stack, Button} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import VerifiablePresentationsList, {
  type VerifiablePresentationsListProps,
} from '../components/VerifiablePresentationsList';
import useVerifiableCredentialRoutes from '../hooks/useVerifiableCredentialRoutes';

export interface VerifiablePresentationsListPageProps extends VerifiablePresentationsListProps {
  /**
   * Whether a presentation definition is created in a project only. When set, a definition can be created
   * inside a selected project and not outside one.
   */
  createInProjectsOnly?: boolean;
}

export default function VerifiablePresentationsListPage({
  showWalletQr = false,
  createInProjectsOnly = false,
}: VerifiablePresentationsListPageProps): JSX.Element {
  const navigate = useNavigate();
  const {projectId} = useProjectScope();
  const canCreate = !createInProjectsOnly || Boolean(projectId);
  const {t} = useTranslation();
  const logger = useLogger('VerifiablePresentationsListPage');
  const routes = useVerifiableCredentialRoutes();

  return (
    <PageContent>
      <DocumentTitle title={t('verifiable-presentations:listing.title', 'Presentation definitions')} />
      <PageTitle>
        <PageTitle.Header>{t('verifiable-presentations:listing.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('verifiable-presentations:listing.subtitle')}{' '}
          <ExternalLink docKey="verifiableCredentials.presentations" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
        {canCreate && (
          <PageTitle.Actions>
            <Stack direction="row" spacing={2}>
              <Button
                data-testid="vp-add-button"
                variant="contained"
                startIcon={<Plus size={18} />}
                onClick={() => {
                  (async () => {
                    await navigate(routes.verifiablePresentations.create());
                  })().catch((error: unknown) => {
                    logger.error('Failed to navigate to create page', {error});
                  });
                }}
              >
                {t('verifiable-presentations:listing.add')}
              </Button>
            </Stack>
          </PageTitle.Actions>
        )}
      </PageTitle>

      <VerifiablePresentationsList showWalletQr={showWalletQr} />
    </PageContent>
  );
}
