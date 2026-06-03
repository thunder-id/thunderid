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

/**
 * Represents a member of a group, which can be a user, group, or app.
 */
export interface Member {
  /** Unique identifier of the member */
  id: string;
  /** Type of the member */
  type: 'user' | 'group' | 'app' | 'agent';
  /** Display name of the member */
  display?: string;
}

/**
 * Represents a group without member details, as returned by list endpoints.
 */
export interface GroupBasic {
  /** Unique identifier of the group */
  id: string;
  /** Display name of the group */
  name: string;
  /** Optional description of the group */
  description?: string;
  /** ID of the organization unit this group belongs to */
  ouId: string;
  /** Handle of the organization unit, populated when include=display is used */
  ouHandle?: string;
  /** Whether this group is read-only (declarative/immutable) */
  isReadOnly?: boolean;
}

/**
 * Represents a group with its full details including members.
 */
export interface Group extends GroupBasic {
  /** Members of this group */
  members?: Member[];
}

/**
 * Paginated response for group list queries.
 */
export interface GroupListResponse {
  /** Total number of groups available */
  totalResults: number;
  /** Starting index of the current page */
  startIndex: number;
  /** Number of groups in the current response */
  count: number;
  /** Array of groups in the current page */
  groups: GroupBasic[];
  /** Pagination links */
  links?: {
    rel: string;
    href: string;
  }[];
}

/**
 * Paginated response for group member list queries.
 */
export interface MemberListResponse {
  /** Total number of members available */
  totalResults: number;
  /** Starting index of the current page */
  startIndex: number;
  /** Number of members in the current response */
  count: number;
  /** Array of members in the current page */
  members: Member[];
  /** Pagination links */
  links?: {
    rel: string;
    href: string;
  }[];
}
