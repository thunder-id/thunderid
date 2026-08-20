// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {Button, PageContent, PageTitle} from '@wso2/oxygen-ui';
import {Plus} from '@wso2/oxygen-ui-icons-react';
import {useState, type JSX} from 'react';
import {useTranslation} from 'react-i18next';
import CreateEnvironmentDialog from '../components/CreateEnvironmentDialog';
import EnvironmentChain from '../components/EnvironmentChain';

/**
 * Page showing the environment promotion chain.
 */
export default function PromotionsListPage(): JSX.Element {
  const {t} = useTranslation();
  const [createOpen, setCreateOpen] = useState<boolean>(false);

  return (
    <PageContent>
      <PageTitle>
        <PageTitle.Header>{t('promotions:listing.title', 'Promotions')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('promotions:listing.subtitle', 'Promote configuration through your environments and review every change')}
        </PageTitle.SubHeader>
        <PageTitle.Actions>
          <Button
            variant="contained"
            startIcon={<Plus size={18} />}
            onClick={() => {
              setCreateOpen(true);
            }}
          >
            {t('promotions:environment.add', 'Add Environment')}
          </Button>
        </PageTitle.Actions>
      </PageTitle>
      <EnvironmentChain />
      <CreateEnvironmentDialog
        open={createOpen}
        onClose={() => {
          setCreateOpen(false);
        }}
      />
    </PageContent>
  );
}
