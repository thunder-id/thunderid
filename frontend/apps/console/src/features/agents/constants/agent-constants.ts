// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * General agent constants.
 */
const AgentConstants = {
  /**
   * Fallback avatar rendered when an agent has no logo set.
   */
  DEFAULT_AVATAR: 'avatar:shape=circle,variant=anonymous_entity,content=bot_head,colors=0',

  /**
   * Length rules the API enforces on the agent name, mirrored here so the console can validate
   * before the request is sent. Keep in sync with `backend/internal/agent/model/agent.go`.
   */
  NAME_MIN_LENGTH: 1,
  NAME_MAX_LENGTH: 100,
} as const;

export default AgentConstants;
