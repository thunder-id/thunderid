// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for user types feature cache management.
 */
const UserTypeQueryKeys = {
  /**
   * Base key for all user type list queries
   */
  USER_TYPES: 'user-types',
  /**
   * Key for a single user type query
   */
  USER_TYPE: 'user-type',
  /**
   * Key for a user type usages query
   */
  USER_TYPE_USAGES: 'user-type-usages',
} as const;

export default UserTypeQueryKeys;
