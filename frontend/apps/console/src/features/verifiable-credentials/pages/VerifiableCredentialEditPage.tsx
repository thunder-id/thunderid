/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {PageLoadingAnimation} from '@thunderid/components';
import {Alert, Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {ArrowLeft} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import {Link, useNavigate, useParams} from 'react-router';
import useGetVerifiableCredential from '../api/useGetVerifiableCredential';
import useUpdateVerifiableCredential from '../api/useUpdateVerifiableCredential';
import VerifiableCredentialDeleteDialog from '../components/VerifiableCredentialDeleteDialog';
import VerifiableCredentialForm from '../components/VerifiableCredentialForm';
import type {UpdateVerifiableCredentialRequest} from '../models/requests';

const LIST_URL = '/verifiable-credentials';

export default function VerifiableCredentialEditPage(): JSX.Element {
  const {vcId = ''} = useParams<{vcId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();

  const {data, isLoading, error} = useGetVerifiableCredential(vcId);
  const updateVC = useUpdateVerifiableCredential();
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  const handleDeleted = (): void => {
    void navigate(LIST_URL);
  };

  const handleSubmit = (formData: UpdateVerifiableCredentialRequest): void => {
    updateVC.mutate({id: vcId, data: formData});
  };

  if (isLoading) {
    return <PageLoadingAnimation />;
  }

  const backButton = (
    <Button
      onClick={(): void => {
        void navigate(LIST_URL);
      }}
      startIcon={<ArrowLeft size={16} />}
    >
      {t('verifiable-credentials:edit.back')}
    </Button>
  );

  if (error) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {error.message ?? t('verifiable-credentials:edit.loadError')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  if (!data) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('verifiable-credentials:edit.notFound')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={LIST_URL} />}>
          {t('verifiable-credentials:edit.back')}
        </PageTitle.BackButton>
        <PageTitle.Header>{data.display?.name ?? data.handle}</PageTitle.Header>
        <PageTitle.SubHeader>{t('verifiable-credentials:edit.subtitle')}</PageTitle.SubHeader>
      </PageTitle>

      <VerifiableCredentialForm
        initial={data}
        submitting={updateVC.isPending}
        submitLabel={t('common:actions.save')}
        onSubmit={handleSubmit}
        onDelete={(): void => setDeleteOpen(true)}
      />

      <VerifiableCredentialDeleteDialog
        open={deleteOpen}
        vcId={vcId}
        onClose={(): void => setDeleteOpen(false)}
        onSuccess={handleDeleted}
      />
    </PageContent>
  );
}
