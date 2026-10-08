// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Notification template channels, matching the channel path segment of the
 * notification template API (GET /notification-templates/{channel}/templates).
 */
export type NotificationTemplateChannel = 'email' | 'sms';

/**
 * List-item representation of a notification template, as returned by the list API.
 */
export interface NotificationTemplateSummary {
  id: string;
  handle: string;
  displayName: string;
  self: string;
}

/**
 * Paginated response for listing a channel's notification templates.
 */
export interface NotificationTemplateListResponse {
  totalResults?: number;
  startIndex?: number;
  count?: number;
  templates: NotificationTemplateSummary[];
}
