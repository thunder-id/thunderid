// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for applications feature cache management.
 */
const ApplicationQueryKeys = {
  /**
   * Base key for all application-related queries
   */
  APPLICATIONS: 'applications',
  /**
   * Key for a single application query
   */
  APPLICATION: 'application',
  /**
   * Key for a quickstart guide fetched from the docs site
   */
  QUICKSTART_GUIDE: 'application-quickstart-guide',
} as const;

export default ApplicationQueryKeys;
