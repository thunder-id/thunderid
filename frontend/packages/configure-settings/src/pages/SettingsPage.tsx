// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink} from '@thunderid/components';
import {Box, PageContent, PageTitle, Tab, Tabs} from '@wso2/oxygen-ui';
import {useState, type JSX, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import AccountSecuritySection from '../components/accountSecurity/AccountSecuritySection';
import CorsSection from '../components/cors/CorsSection';

export default function SettingsPage(): JSX.Element {
  const {t} = useTranslation();
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
        <Tab
          label={t('settings:tabs.accountSecurity', 'Account security')}
          id="settings-tab-1"
          aria-controls="settings-tabpanel-1"
          sx={{textTransform: 'none'}}
        />
      </Tabs>

      {/* Both panels stay mounted so unsaved drafts survive a tab switch. */}
      <Box
        role="tabpanel"
        id="settings-tabpanel-0"
        aria-labelledby="settings-tab-0"
        hidden={activeTab !== 0}
        sx={{py: 3}}
      >
        <CorsSection />
      </Box>
      <Box
        role="tabpanel"
        id="settings-tabpanel-1"
        aria-labelledby="settings-tab-1"
        hidden={activeTab !== 1}
        sx={{py: 3}}
      >
        <AccountSecuritySection />
      </Box>
    </PageContent>
  );
}
