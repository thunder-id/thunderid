// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for resource sharing cache management.
 */
const ResourceSharingQueryKeys = {
  /**
   * Base key for one resource's sharing policies, followed by the resource's path
   */
  SHARING_POLICIES: 'sharing-policies',
  /**
   * Base key for the roles an issuer can mandate in one unit, followed by the unit and the issuer
   */
  MANDATABLE_ROLES: 'mandatable-roles',
} as const;

export default ResourceSharingQueryKeys;
