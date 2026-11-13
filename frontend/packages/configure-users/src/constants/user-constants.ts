// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * General user constants.
 */
const UserConstants = {
  /**
   * Fallback avatar prefix rendered when a user has no picture set. Append the user's initials as `content`.
   */
  DEFAULT_AVATAR_PREFIX: 'avatar:shape=circle,variant=two_letter,colors=0,content=',
  /**
   * Fallback avatar rendered when an owned agent has no logo set, the same one the agent pages use.
   */
  DEFAULT_AGENT_AVATAR: 'avatar:shape=circle,variant=anonymous_entity,content=bot_head,colors=0',
  /**
   * Kind of an agent a user registered for themselves.
   */
  PERSONAL_AGENT_KIND: 'personal-assistant',
  /**
   * Flow type of the administrator-initiated attribute verification flow.
   */
  ADMIN_VERIFICATION_FLOW_TYPE: 'ADMIN_INITIATED',
  /**
   * Schema property types an administrator can send a verification link for. The shipped flow sends email only.
   */
  ADMIN_VERIFIABLE_TYPES: ['email'] as readonly string[],
} as const;

export default UserConstants;
