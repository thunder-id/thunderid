// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Alert, Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus, Upload} from '@wso2/oxygen-ui-icons-react';
import {type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {useNavigate, useParams} from 'react-router';
import useApplyAll from '../../promotions/api/useApplyAll';
import EnvironmentVariablesList from '../components/EnvironmentVariablesList';

/**
 * Page listing the environment variables with a create action.
 */
export default function EnvironmentVariablesListPage(): JSX.Element {
  const {envId = ''} = useParams<{envId: string}>();
  const {t} = useTranslation();
  const navigate = useNavigate();
  const applyAll = useApplyAll();

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('environmentVariables:listing.title', 'Environment Variables')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t(
            'environmentVariables:listing.subtitle',
            'Non-secret values substituted into configuration when it is applied to a Data Plane',
          )}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            startIcon={<Upload size={18} />}
            disabled={applyAll.isPending}
            onClick={() => {
              applyAll.mutate();
            }}
          >
            {applyAll.isPending
              ? t('promotions:applyAll.inProgress', 'Applying...')
              : t('promotions:applyAll.action', 'Apply to Data Planes')}
          </Button>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              void navigate(`/promotions/${envId}/variables/create`);
            }}
          >
            {t('environmentVariables:listing.add', 'Add Variable')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>
      <Alert severity="info" sx={{mb: 2}}>
        {t(
          'environmentVariables:applyNotice',
          'A change here reaches a Data Plane only when configuration is applied. Use Apply to Data Planes to push it now.',
        )}
      </Alert>
      <EnvironmentVariablesList />
    </PageContent>
  );
}
