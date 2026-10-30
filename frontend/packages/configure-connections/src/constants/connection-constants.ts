// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * General connections constants.
 */
const ConnectionConstants = {
  /**
   * Avatar rendered for trusted issuer connection cards.
   */
  DEFAULT_TRUSTED_IDP_AVATAR: 'avatar:shape=rounded,variant=anonymous_entity,content=chevron,colors=0',

  /**
   * Fallback avatar rendered for the OpenID Connect vendor.
   */
  OIDC_AVATAR_FALLBACK: 'avatar:shape=rounded,variant=anonymous_entity,content=triangle_stack,colors=0',

  /**
   * Fallback avatar rendered for the OAuth 2 vendor.
   */
  OAUTH_AVATAR_FALLBACK: 'avatar:shape=rounded,variant=anonymous_entity,content=parallelogram,colors=0',

  /**
   * Fallback avatar rendered for the SMS Gateway vendor.
   */
  SMS_GATEWAY_AVATAR_FALLBACK: 'avatar:shape=rounded,variant=anonymous_entity,content=pentagon,colors=0',

  /**
   * The mode each sync setting takes on the server when it is unset.
   */
  DEFAULT_SYNC_MODES: {attributes: 'override_received', groupsAndRoles: 'no_sync'},
} as const;

export default ConnectionConstants;
