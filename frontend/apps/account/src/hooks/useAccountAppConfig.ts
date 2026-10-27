// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {DEFAULT_ACCOUNT_APP_CONFIG, useGetAppConfig, type AppConfig, type AppPageConfig} from '@thunderid/system-apps';
import {ACCOUNT_PAGES, type AccountPage} from '../configs/RouteConfig';

/**
 * The Account App configuration the Console saved.
 */
export interface AccountAppConfig {
  config: AppConfig;
  /**
   * Whether the configuration is still loading. The default configuration is used meanwhile.
   */
  isLoading: boolean;
  /**
   * The configuration of one page. A page the configuration does not name is treated as off.
   */
  page: (key: AccountPage) => AppPageConfig | undefined;
  /**
   * The pages that are on, in navigation order.
   */
  visiblePages: AccountPage[];
}

/**
 * Reads the Account App configuration saved in the Console, falling back to the default
 * configuration while it loads or when it cannot be read, so the app stays usable.
 */
export default function useAccountAppConfig(): AccountAppConfig {
  const {data, isLoading} = useGetAppConfig('ACCOUNT');
  const config: AppConfig = data ?? DEFAULT_ACCOUNT_APP_CONFIG;
  const page = (key: AccountPage): AppPageConfig | undefined =>
    config.pages.find((candidate: AppPageConfig) => candidate.key === key);

  return {
    config,
    isLoading,
    page,
    visiblePages: ACCOUNT_PAGES.filter((key: AccountPage) => page(key)?.enabled === true),
  };
}
