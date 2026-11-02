// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {useLogger} from '@thunderid/logger/react';
import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useTranslation} from 'react-i18next';
import {useNavigate} from 'react-router';
import UserTypesList from '../components/UserTypesList';
import useUserTypeRoutes from '../hooks/useUserTypeRoutes';

export default function UserTypesListPage() {
  const navigate = useNavigate();
  const {t} = useTranslation();
  const logger = useLogger('UserTypesListPage');
  const routes = useUserTypeRoutes();

  return (
    <PageContent>
      <DocumentTitle title={t('userTypes:title', 'User types')} />
      {/* Header */}
      <PageTitle>
        <PageTitle.Header>{t('userTypes:title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('userTypes:subtitle')} <ExternalLink docKey="userTypes" />
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              const handler = async () => {
                await navigate(routes.create());
              };

              handler().catch((error: unknown) => {
                logger.error('Failed to navigate to create user type page', {error});
              });
            }}
          >
            {t('userTypes:createUserType')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>

      <UserTypesList />
    </PageContent>
  );
}
