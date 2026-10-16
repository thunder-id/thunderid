// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Effective access status of an identity, derived by the server at read time with precedence
 * `SUSPENDED` > `LOCKED` > `ACTIVE`. Unsuspending does not clear a lock. Applications are never `LOCKED`.
 *
 * @public
 */
export type AccessStatus = 'ACTIVE' | 'LOCKED' | 'SUSPENDED';

/**
 * The lifecycle operations an operator can run against an identity. Locking is automatic only.
 *
 * @public
 */
export type LifecycleOperation = 'unlock' | 'suspend' | 'unsuspend';

/**
 * The identity categories the lifecycle operations apply to.
 *
 * @public
 */
export type LifecycleCategory = 'user' | 'agent' | 'application';

/**
 * What put an identity into the status it reports. Reported on a management read only.
 *
 * @public
 */
export type AccessHoldReason = 'FAILED_ATTEMPTS' | 'SUSPENSION_RELEASE_LOCK' | 'ADMINISTRATIVE_SUSPENSION';

/**
 * One live automatic lock: the authentication method it holds, and when it lifts.
 *
 * @public
 */
export interface AccessLockedScope {
  /**
   * The authentication method this lock is held against, or `entity` when it holds every authentication method.
   */
  scope: string;
  /**
   * When this lock lifts on its own. Absent when it lasts until released.
   */
  expiresAt?: string;
}

/**
 * What a management read says about a status beyond its name. Absent when there is nothing to add.
 *
 * @public
 */
export interface AccessStatusDetails {
  /**
   * What produced the status.
   */
  reason?: AccessHoldReason;
  /**
   * When an administrative hold was placed. Absent for an automatic lock.
   */
  since?: string;
  /**
   * The operator's optional note on the hold. Render as text, never as markup.
   */
  operatorNote?: string;
  /**
   * Every automatic lock currently live, in the authentication method registry's order. Absent for a status that
   * is not a lock.
   */
  lockedScopes?: AccessLockedScope[];
}

/**
 * The status a management read carries: the name, and what is behind it.
 *
 * @public
 */
export interface AccessStatusReport {
  /**
   * The effective status.
   */
  value: AccessStatus;
  /**
   * What produced it and what will clear it. Absent when there is nothing to add.
   */
  details?: AccessStatusDetails;
}
