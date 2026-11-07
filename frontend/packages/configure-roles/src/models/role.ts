// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ResourcePermissions} from '@thunderid/configure-resource-servers';

export type {ResourcePermissions};

/**
 * An assignment of a user, group, or app to a role.
 */
export interface RoleAssignment {
  /** Unique identifier of the user or group */
  id: string;
  /** Type of assignee */
  type: 'user' | 'group' | 'app' | 'agent';
  /** Display name (resolved when include=display is used) */
  display?: string;
  /** True when a user share mandates the assignment, so the organization unit cannot remove it. */
  isReadOnly?: boolean;
}

/**
 * Summary representation of a role as returned in list responses.
 */
export interface RoleSummary {
  /** Unique identifier of the role */
  id: string;
  /** Name of the role */
  name: string;
  /** Optional description */
  description?: string;
  /** ID of the organization unit this role belongs to */
  ouId: string;
  /** Handle of the organization unit (resolved when include=display is used) */
  ouHandle?: string;
  /** Whether this role is read-only (declarative/immutable) */
  isReadOnly?: boolean;
}

/**
 * Full role details including permissions.
 */
export interface Role extends RoleSummary {
  /** Permissions grouped by resource server */
  permissions?: ResourcePermissions[];
}

/**
 * Paginated response for role list queries.
 */
export interface RoleListResponse {
  /** Total number of roles available */
  totalResults: number;
  /** Starting index of the current page */
  startIndex: number;
  /** Number of roles in the current response */
  count: number;
  /** Array of roles in the current page */
  roles: RoleSummary[];
  /** Pagination links */
  links?: {
    rel: string;
    href: string;
  }[];
}

/**
 * Paginated response for role assignment list queries.
 */
export interface RoleAssignmentListResponse {
  /** Total number of assignments */
  totalResults: number;
  /** Starting index of the current page */
  startIndex: number;
  /** Number of assignments in the current response */
  count: number;
  /** Array of assignments */
  assignments: RoleAssignment[];
  /** Pagination links */
  links?: {
    rel: string;
    href: string;
  }[];
}
