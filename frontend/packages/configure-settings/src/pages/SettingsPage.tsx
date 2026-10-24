// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {Box, PageContent, PageTitle, Tab, Tabs} from '@wso2/oxygen-ui';
import {useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import CorsSection from '../components/cors/CorsSection';

export default function SettingsPage(): JSX.Element {
  const {t} = useTranslation();
  // Controlled Tabs; only the CORS tab exists for now.
  const [activeTab, setActiveTab] = useState(0);

  const handleTabChange = (_event: SyntheticEvent, newValue: number): void => {
    setActiveTab(newValue);
  };

  return (
    <PageContent>
      <DocumentTitle title={t('settings:page.title', 'Settings')} />
      <PageTitle>
        <PageTitle.Header>{t('settings:page.title')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('settings:page.subtitle')} <ExternalLink docKey="settings" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
      </PageTitle>

      <Tabs value={activeTab} onChange={handleTabChange} aria-label={t('settings:tabs.ariaLabel')}>
        <Tab
          label={t('settings:tabs.cors')}
          id="settings-tab-0"
          aria-controls="settings-tabpanel-0"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      <Box role="tabpanel" id="settings-tabpanel-0" aria-labelledby="settings-tab-0" sx={{py: 3}}>
        <CorsSection />
      </Box>
    </PageContent>
  );
}
