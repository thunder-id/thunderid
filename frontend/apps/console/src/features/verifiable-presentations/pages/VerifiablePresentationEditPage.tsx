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
import useGetVerifiablePresentation from '../api/useGetVerifiablePresentation';
import useUpdateVerifiablePresentation from '../api/useUpdateVerifiablePresentation';
import VerifiablePresentationDeleteDialog from '../components/VerifiablePresentationDeleteDialog';
import VerifiablePresentationForm from '../components/VerifiablePresentationForm';
import type {UpdateVerifiablePresentationRequest} from '../models/requests';

const LIST_URL = '/verifiable-presentations';

export default function VerifiablePresentationEditPage(): JSX.Element {
  const {vpId = ''} = useParams<{vpId: string}>();
  const navigate = useNavigate();
  const {t} = useTranslation();

  const {data, isLoading, error} = useGetVerifiablePresentation(vpId);
  const updateVP = useUpdateVerifiablePresentation();
  const [deleteOpen, setDeleteOpen] = useState<boolean>(false);

  const handleDeleted = (): void => {
    void navigate(LIST_URL);
  };

  const handleSubmit = (formData: UpdateVerifiablePresentationRequest): void => {
    // Save in place — the form re-snapshots from the refreshed query and the
    // success toast confirms; no navigation away (UnsavedChangesBar pattern).
    updateVP.mutate({id: vpId, data: formData});
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
      {t('verifiable-presentations:edit.back')}
    </Button>
  );

  if (error) {
    return (
      <PageContent>
        <Alert severity="error" sx={{mb: 2}}>
          {error.message ?? t('verifiable-presentations:edit.loadError')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  if (!data) {
    return (
      <PageContent>
        <Alert severity="warning" sx={{mb: 2}}>
          {t('verifiable-presentations:edit.notFound')}
        </Alert>
        {backButton}
      </PageContent>
    );
  }

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.BackButton component={<Link to={LIST_URL} />}>
          {t('verifiable-presentations:edit.back')}
        </PageTitle.BackButton>
        <PageTitle.Header>{data.displayName ?? data.handle}</PageTitle.Header>
        <PageTitle.SubHeader>{t('verifiable-presentations:edit.subtitle')}</PageTitle.SubHeader>
      </PageTitle>

      <VerifiablePresentationForm
        initial={data}
        submitting={updateVP.isPending}
        submitLabel={t('common:actions.save')}
        onSubmit={handleSubmit}
        onDelete={(): void => setDeleteOpen(true)}
      />

      <VerifiablePresentationDeleteDialog
        open={deleteOpen}
        vpId={vpId}
        onClose={(): void => setDeleteOpen(false)}
        onSuccess={handleDeleted}
      />
    </PageContent>
  );
}
