// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ServerConfigLayers} from './responses';

/**
 * What an automatic lock holds.
 *
 * `authentication_method` holds only the sign-in method that failed, so wrong passwords do not stop a one-time
 * code. `entity` folds every countable failure into one counter and holds every sign-in method.
 *
 * The wire values are the server's and are not renamed here: `entity` is also the reserved key the
 * shared counter is stored under, and the two are the same string by design. The Console renders
 * labels, never these.
 *
 * @public
 */
export type LockGranularity = 'authentication_method' | 'entity';

/**
 * The lockout policy of one identity category.
 *
 * Every field is optional because an omitted field inherits the layer beneath it, and omitting is not
 * the same as setting zero: a window of `0` never resets the failure count on time, a lock duration
 * of `0` is held until an operator unlocks, and a decay of `0` never resets the escalation.
 *
 * @public
 */
export interface AccountAccessPolicy {
  enabled?: boolean;
  threshold?: number;
  failureWindowSeconds?: number;
  lockDurationsSeconds?: number[];
  lockDecaySeconds?: number;
}

/**
 * The account access settings for one identity category.
 *
 * @public
 */
export interface AccountAccessCategory {
  lockGranularity?: LockGranularity;
  /** Whether a successful sign-in records `lastLoginAt`. Off unless explicitly enabled. */
  recordLastLogin?: boolean;
  /** How stale the recorded `lastLoginAt` may be before a sign-in writes a new one. `0` writes every time. */
  lastLoginResolutionSeconds?: number;
  default?: AccountAccessPolicy;
  notifications?: AccountAccessNotifications;
}

/**
 * What one category tells the account holder when a lock forms.
 *
 * A sibling of the lockout policy rather than a field inside it: switching the email off must not
 * change a threshold, and switching locking off must not silently drop a notice. Only `user` sends
 * these, so the Console shows them on that card alone.
 *
 * @public
 */
export interface AccountAccessNotifications {
  onLock?: {
    email?: AccountAccessLockEmail;
  };
}

/**
 * The automatic-lock email for one category.
 *
 * The email provider is not here: the notice goes through the default email sender of the
 * `notification` server-config section. The Console edits the recipient attribute while preserving
 * fields the operator has not changed.
 *
 * @public
 */
export interface AccountAccessLockEmail {
  enabled?: boolean;
  recipientAttribute?: string;
}

/**
 * The value of the `accountAccess` server-config section.
 *
 * @public
 */
export interface AccountAccessValue {
  user?: AccountAccessCategory;
  agent?: AccountAccessCategory;
}

/**
 * The categories the Console offers automatic-lock settings for.
 *
 * Applications are absent on purpose: they resolve only at the application plane, where a credential
 * hold is never enforced, so an application can never reach the locked state by any route.
 *
 * @public
 */
export type AccountLockCategory = 'user' | 'agent';

/**
 * The categories in the order the settings page shows them.
 *
 * @public
 */
export const AccountLockCategories: readonly AccountLockCategory[] = ['user', 'agent'] as const;

/**
 * The `accountAccess` section as its three layers.
 *
 * @public
 */
export type AccountAccessConfigResponse = ServerConfigLayers<AccountAccessValue>;
