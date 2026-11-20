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
  /**
   * Key for a coding agent prompt fetched from the docs site
   */
  CODING_AGENT_PROMPT: 'application-coding-agent-prompt',
  /**
   * Key for the released archive of a sample app
   */
  SAMPLE_RELEASE: 'application-sample-release',
  /**
   * Key for what an application's field holds in one environment
   */
  ENVIRONMENT_VALUE: 'application-environment-value',
} as const;

export default ApplicationQueryKeys;
