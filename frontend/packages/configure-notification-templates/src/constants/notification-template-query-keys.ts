// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Query key constants for notification-template feature cache management.
 */
const NotificationTemplateQueryKeys = {
  /**
   * Base key for all notification-template list queries.
   */
  TEMPLATES: 'notification-templates',
  /**
   * Key for a single notification-template query.
   */
  TEMPLATE: 'notification-template',
} as const;

export default NotificationTemplateQueryKeys;
