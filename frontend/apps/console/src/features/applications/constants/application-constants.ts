// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * General application constants.
 */
const ApplicationConstants = {
  /**
   * Fallback avatar rendered when an application has no logo set.
   */
  DEFAULT_AVATAR: 'avatar:shape=rounded,variant=anonymous_entity,content=cube,colors=0',

  /**
   * Backend error code (HTTP 400 body code) returned when an application name is already in use.
   */
  DUPLICATE_APP_NAME_ERROR_CODE: 'APP-1020',
} as const;

export default ApplicationConstants;
