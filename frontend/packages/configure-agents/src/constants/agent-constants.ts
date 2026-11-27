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
} as const;

export default AgentConstants;

/** How often the agent is read again while a sent owner verification link waits for the owner. */
export const OWNER_VERIFICATION_POLL_MS = 5000;

/** How long after a send the agent is read again on its own; after that, the administrator checks by hand. */
export const OWNER_VERIFICATION_POLL_LIMIT_MS = 10 * 60 * 1000;
