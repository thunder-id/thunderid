// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useThunderID} from '@thunderid/react';
import {DEFAULT_ADMIN_APP_CONFIG, useGetAppConfig, type AppConfig} from '@thunderid/system-apps';

export interface AdminAppConfig {
  /**
   * The Admin App configuration the Console's builder saved, or the app's defaults until it loads
   * and whenever it cannot be read.
   */
  config: AppConfig;
  /**
   * Whether the configuration is still loading.
   */
  isLoading: boolean;
}

/**
 * Reads the Admin App configuration from the server once the admin is signed in, since only a
 * signed-in user may read it.
 */
export default function useAdminAppConfig(): AdminAppConfig {
  const {isSignedIn} = useThunderID();
  const {data, isLoading} = useGetAppConfig('ADMIN', {enabled: Boolean(isSignedIn)});

  return {config: data ?? DEFAULT_ADMIN_APP_CONFIG, isLoading};
}
