// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DocumentTitle, ExternalLink, PageLoadingAnimation, QueryErrorNotice} from '@thunderid/components';
import {Box, PageContent, PageTitle, Tab, Tabs} from '@wso2/oxygen-ui';
import {useCallback, type JSX, type ReactNode, type SyntheticEvent} from 'react';
import {useTranslation} from 'react-i18next';
import {useSearchParams} from 'react-router';
import useGetServerConfigSections from '../api/useGetServerConfigSections';
import AccountSecuritySection from '../components/accountSecurity/AccountSecuritySection';
import CorsSection from '../components/cors/CorsSection';
import SessionTimeoutsCard from '../components/sessions/SessionTimeoutsCard';
import SettingsQueryKeys from '../constants/settings-query-keys';

/** The query parameter that holds the active tab. */
const TAB_PARAM = 'tab';

interface SettingsTab {
  /** The tab's value in the URL. */
  value: string;
  /** The server-config section the tab edits. The tab shows only when the server serves it. */
  section: string;
  label: string;
  panel: ReactNode;
}

export default function SettingsPage(): JSX.Element {
  const {t} = useTranslation();
  const {data: sections, error, refetch} = useGetServerConfigSections();
  const [searchParams, setSearchParams] = useSearchParams();

  const tForErrors = useCallback(
    (key: string, options?: Record<string, unknown>): string => t(key.includes(':') ? key : `settings:${key}`, options),
    [t],
  );

  const tabs: SettingsTab[] = [
    {
      value: 'account-security',
      section: SettingsQueryKeys.ACCOUNT_ACCESS,
      label: t('settings:tabs.accountSecurity', 'Account security'),
      panel: <AccountSecuritySection />,
    },
    {
      value: 'cors',
      section: SettingsQueryKeys.CORS,
      label: t('settings:tabs.cors', 'CORS'),
      panel: <CorsSection />,
    },
    {
      value: 'sessions',
      section: SettingsQueryKeys.SESSION,
      label: t('settings:tabs.sessions', 'Sign-in session'),
      panel: <SessionTimeoutsCard />,
    },
  ].filter((tab) => sections?.includes(tab.section));

  // A missing or unknown tab, or one this server does not serve, opens the first tab shown.
  const requestedTab: string | null = searchParams.get(TAB_PARAM);
  const activeTab: string | undefined = (tabs.find((tab) => tab.value === requestedTab) ?? tabs.at(0))?.value;

  const handleTabChange = (_event: SyntheticEvent, value: string): void => {
    // Replaced rather than pushed, so Back leaves the page instead of stepping through its tabs.
    setSearchParams(
      (current) => {
        const next = new URLSearchParams(current);
        next.set(TAB_PARAM, value);
        return next;
      },
      {replace: true},
    );
  };

  let body: JSX.Element;
  if (sections) {
    body = (
      <>
        {/* A later read of the list that failed keeps the tabs read before, and every draft in them. */}
        {error && (
          <Box sx={{mb: 2}}>
            <QueryErrorNotice
              error={error}
              t={tForErrors}
              variant="inline"
              fallbackKey="settings:page.refresh.error"
              fallbackDefaultValue="Failed to refresh the configurations. The tabs show what was loaded before."
              onRetry={() => void refetch()}
            />
          </Box>
        )}
        <Tabs
          value={activeTab ?? false}
          onChange={handleTabChange}
          aria-label={t('settings:tabs.ariaLabel', 'Settings sections')}
        >
          {tabs.map((tab) => (
            <Tab
              key={tab.value}
              value={tab.value}
              label={tab.label}
              id={`settings-tab-${tab.value}`}
              aria-controls={`settings-tabpanel-${tab.value}`}
              sx={{textTransform: 'none'}}
            />
          ))}
        </Tabs>

        {/* Every panel stays mounted so unsaved drafts survive a tab switch. */}
        {tabs.map((tab) => (
          <Box
            key={tab.value}
            role="tabpanel"
            id={`settings-tabpanel-${tab.value}`}
            aria-labelledby={`settings-tab-${tab.value}`}
            hidden={tab.value !== activeTab}
            sx={{py: 3}}
          >
            {tab.panel}
          </Box>
        ))}
      </>
    );
  } else if (error) {
    body = (
      <QueryErrorNotice
        error={error}
        t={tForErrors}
        title={t('settings:page.load.error', 'Failed to load the configurations')}
        onRetry={() => void refetch()}
      />
    );
  } else {
    body = <PageLoadingAnimation />;
  }

  return (
    <PageContent>
      <DocumentTitle title={t('settings:page.title', 'Configurations')} />
      <PageTitle>
        <PageTitle.Header>{t('settings:page.title', 'Configurations')}</PageTitle.Header>
        <PageTitle.SubHeader>
          {t('settings:page.subtitle', 'Configurations that apply across your system.')}{' '}
          <ExternalLink docKey="settings" confirmBeforeNavigate={false} />
        </PageTitle.SubHeader>
      </PageTitle>

      {body}
    </PageContent>
  );
}
