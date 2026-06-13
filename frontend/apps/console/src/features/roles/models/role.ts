/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

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
