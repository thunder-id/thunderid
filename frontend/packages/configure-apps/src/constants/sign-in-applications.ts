// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AppType} from '@thunderid/system-apps';

/**
 * The application each app signs in with, as the default resources create it. Its Gate theme and
 * layout style the app's sign-in screens.
 */
const SIGN_IN_APPLICATION_IDS: Record<AppType, string> = {
  ACCOUNT: '01900000-0000-7000-8000-0000000000f6',
  ADMIN: '01900000-0000-7000-8000-0000000000f1',
};

export default SIGN_IN_APPLICATION_IDS;
