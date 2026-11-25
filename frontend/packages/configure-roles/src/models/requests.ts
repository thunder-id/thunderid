// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ResourcePermissions, RoleAssignment} from './role';

/**
 * Request payload for creating a new role.
 */
export interface CreateRoleRequest {
  /** Name of the role */
  name: string;
  /** Optional description */
  description?: string;
  /** ID of the organization unit this role belongs to; the system organization unit when omitted */
  ouId?: string;
  /** Places the role in the project, in the system organization unit unless `ouId` names one of the project's */
  projectId?: string;
  /** Optional initial permissions */
  permissions?: ResourcePermissions[];
  /** Optional initial assignments */
  assignments?: RoleAssignment[];
}

/**
 * Request payload for updating a role.
 */
export interface UpdateRoleRequest {
  /** Name of the role */
  name: string;
  /** Optional description */
  description?: string;
  /** ID of the organization unit */
  ouId: string;
  /** Full permissions list (replaces existing) */
  permissions: ResourcePermissions[];
}

/**
 * Request payload for adding or removing role assignments.
 */
export interface AssignmentsRequest {
  /** List of assignments to add or remove */
  assignments: RoleAssignment[];
}

/**
 * Pagination parameters for role list queries.
 */
export interface RoleListParams {
  /** Maximum number of records to return */
  limit?: number;
  /** Number of records to skip */
  offset?: number;
  /** Lists only the roles of this project */
  projectId?: string;
}

/**
 * Pagination parameters for role assignment list queries.
 */
export interface RoleAssignmentListParams {
  /** Maximum number of records to return */
  limit?: number;
  /** Number of records to skip */
  offset?: number;
  /** Set to "display" to resolve display names */
  include?: 'display';
  /** Filter by assignment type */
  type?: 'user' | 'group' | 'app' | 'agent';
}
