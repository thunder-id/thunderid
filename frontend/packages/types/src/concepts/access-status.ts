// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Effective access status, with precedence `SUSPENDED` > `EXPIRED` > `LOCKED` > `ACTIVE`. Applications are never
 * `LOCKED`. `EXPIRED` means the identity's lifespan has ended.
 *
 * @public
 */
export type AccessStatus = 'ACTIVE' | 'LOCKED' | 'SUSPENDED' | 'EXPIRED';

/**
 * What put an identity into the status it reports. Reported on a management read only.
 *
 * @public
 */
export type AccessHoldReason =
  | 'FAILED_ATTEMPTS'
  | 'SUSPENSION_RELEASE_LOCK'
  | 'ADMINISTRATIVE_SUSPENSION'
  | 'LIFESPAN_ENDED';

/**
 * One live automatic lock: the authentication method it holds, and when it lifts.
 *
 * @public
 */
export interface AccessLockedMethod {
  /**
   * The authentication method this lock holds, or `account` for all of them.
   */
  method: string;
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
   * When an administrative hold was placed, or when the lifespan ended. Absent for an automatic lock.
   */
  since?: string;
  /**
   * The operator's optional note on the hold. Render as text, never as markup.
   */
  operatorNote?: string;
  /**
   * Every live automatic lock. Absent for a status that is not a lock.
   */
  lockedMethods?: AccessLockedMethod[];
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
