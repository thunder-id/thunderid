// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {NotificationChannel} from '@thunderid/configure-design';

/**
 * Notification template channels, matching the channel path segment of the
 * notification template API (GET /notification-templates/{channel}/templates).
 *
 * Aliased to `@thunderid/configure-design`'s `NotificationChannel` so the channel union has a single
 * definition shared across the notification, design, and flows packages.
 */
export type NotificationTemplateChannel = NotificationChannel;

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
