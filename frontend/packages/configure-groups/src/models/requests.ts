// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {Member} from './group';

/**
 * Request payload for creating a new group.
 */
export interface CreateGroupRequest {
  /** Display name of the group */
  name: string;
  /** Optional description */
  description?: string;
  /** ID of the organization unit this group belongs to */
  ouId: string;
  /** Optional initial members */
  members?: Member[];
}

/**
 * Request payload for updating a group.
 */
export interface UpdateGroupRequest {
  /** Display name of the group */
  name: string;
  /** Optional description */
  description?: string;
  /** ID of the organization unit this group belongs to */
  ouId: string;
  /** Members of the group */
  members?: Member[];
}
/**
 * Pagination parameters for group list queries.
 */
export interface GroupListParams {
  /** Maximum number of records to return */
  limit?: number;
  /** Number of records to skip */
  offset?: number;
  /** Filter expression, for example `ouId eq "<id>"` */
  filter?: string;
}
