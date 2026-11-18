// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for users feature cache management.
 */
const UserQueryKeys = {
  /**
   * Base key for user list queries.
   */
  USERS: 'users',
  /**
   * Key for a single user query.
   */
  USER: 'user',
  /**
   * Key for a user usages query.
   */
  USER_USAGES: 'user-usages',
  /**
   * Key for a user's owned agents query.
   */
  USER_OWNED_AGENTS: 'user-owned-agents',
  /**
   * Key for the roles assigned to a user.
   */
  USER_ROLES: 'user-roles',
  /**
   * Key for the roles a user can be given.
   */
  ASSIGNABLE_ROLES: 'user-assignable-roles',
  /**
   * Key for the handle of the attribute verification flow the server configures.
   */
  ATTRIBUTE_VERIFICATION_FLOW: 'attribute-verification-flow',
  /**
   * Base key for user type list queries.
   */
  USER_TYPES: 'userTypes',
  /**
   * Key for a single user type query.
   */
  USER_TYPE: 'userType',
} as const;

export default UserQueryKeys;
