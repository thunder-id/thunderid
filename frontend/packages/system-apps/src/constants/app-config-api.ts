// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppType} from '../models/app-config';

/**
 * Query keys for app configurations.
 *
 * @public
 */
export const AppConfigQueryKeys = {
  APP_CONFIG: 'app-config',
} as const;

/**
 * The path of an app's server-config section: `/server-config/system-apps/admin` for the Admin App and
 * `/server-config/system-apps/account` for the Account App.
 *
 * @public
 */
export const appConfigPath = (type: AppType): string =>
  `/server-config/system-apps/${type === 'ACCOUNT' ? 'account' : 'admin'}`;
