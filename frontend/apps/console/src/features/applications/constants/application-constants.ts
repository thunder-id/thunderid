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

  /**
   * The only agent type the API accepts, written to `allowedAgentTypes` when agent sign-in is
   * enabled for an application. Keep in sync with `backend/internal/entitytype/model.go`.
   */
  DEFAULT_AGENT_TYPE: 'default',

  /**
   * Length rules the API enforces on the application name, mirrored here so the console can
   * validate before the request is sent. Keep in sync with
   * `backend/internal/application/model/application.go`.
   */
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 100,
} as const;

export default ApplicationConstants;
