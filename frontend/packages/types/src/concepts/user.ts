// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {AccessStatusReport} from './access-status';

/**
 * User representation.
 *
 * Describes the core user shape shared across frontend packages
 * for identity, organizational context, and profile data.
 *
 * @public
 */
export interface User {
  /**
   * Unique identifier of the user.
   */
  id: string;

  /**
   * Identifier of the organizational unit the user belongs to.
   */
  ouId: string;

  /**
   * Handle of the organizational unit, when available.
   */
  ouHandle?: string;

  /**
   * User category or classification.
   */
  type: string;

  /**
   * Effective access status and what produced it, derived by the server at read time rather than
   * stored.
   *
   * Read-only. It is never accepted on a write: the lifecycle operations are the only way to change
   * it, and a lock is not settable at all. Absent on responses from a server that predates it.
   *
   * The details inside are reported on this read and never on an authentication refusal: a caller
   * already authorized to see the record is a different audience from one probing usernames.
   */
  status?: AccessStatusReport;
  /**
   * When this user was last granted access, session reuse included. Absent until a sign-in has been
   * recorded.
   */
  lastLoginAt?: string;

  /**
   * Additional user attributes returned by the API.
   */
  attributes?: Record<string, unknown>;

  /**
   * Human-readable display name for the user.
   */
  display?: string;

  /**
   * Whether the user is read-only and cannot be modified or deleted.
   */
  isReadOnly?: boolean;
}
