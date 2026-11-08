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
   * Identifier of the project the user belongs to; absent when the user belongs to the organization.
   */
  projectId?: string;

  /**
   * User category or classification.
   */
  type: string;

  /**
   * Effective access status, derived by the server at read time. Read-only.
   */
  status?: AccessStatusReport;

  /**
   * Additional user attributes returned by the API.
   */
  attributes?: Record<string, unknown>;

  /**
   * When each current contact attribute was verified, keyed by attribute. Empty when none is verified; absent when the
   * server did not read it, so the status is unknown.
   */
  readonly verifiedAttributes?: Readonly<Record<string, {readonly verifiedAt: string}>>;

  /**
   * Human-readable display name for the user.
   */
  display?: string;

  /**
   * Whether the user is read-only and cannot be modified or deleted.
   */
  isReadOnly?: boolean;
}
