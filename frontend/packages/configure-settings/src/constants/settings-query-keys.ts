// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for the settings feature cache management.
 */
const SettingsQueryKeys = {
  /** Base key for all server-config section queries */
  SERVER_CONFIG: 'server-config',
  /** Name segment for the CORS section */
  CORS: 'cors',
  /** Name segment for the account-lockout section */
  ACCOUNT_ACCESS: 'accountAccess',
  /** Base key for the resolved account lifecycle administration flows */
  ACCOUNT_LIFECYCLE_FLOWS: 'account-lifecycle-flows',
} as const;

export default SettingsQueryKeys;
