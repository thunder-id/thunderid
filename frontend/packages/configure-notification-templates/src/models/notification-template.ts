// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {NotificationChannel} from '@thunderid/configure-design';

/**
 * Notification channel a template belongs to. Re-exported from `@thunderid/configure-design`, which
 * owns the shared token grammar, so the type has a single definition across packages.
 *
 * @public
 */
export type {NotificationChannel};

/** Type guard narrowing an unknown route param to a supported {@link NotificationChannel}. */
export function isNotificationChannel(value: string | undefined): value is NotificationChannel {
  return value === 'email' || value === 'sms';
}

/**
 * Color scheme the email template design is authored against.
 *
 * @public
 */
export type TemplateColorScheme = 'light' | 'dark';

/**
 * Optional design metadata for an email template.
 *
 * @public
 */
export interface TemplateDesign {
  /**
   * Color scheme the template body is authored against.
   */
  colorScheme?: TemplateColorScheme;
}

/**
 * Content of an email template.
 *
 * @public
 */
export interface EmailTemplateContent {
  /**
   * Subject line of the email.
   */
  subject: string;
  /**
   * HTML body of the email.
   */
  body: string;
}

/**
 * Content of an SMS template.
 *
 * @public
 */
export interface SmsTemplateContent {
  /**
   * Text body of the SMS.
   */
  body: string;
}

/**
 * A single email notification template.
 *
 * @public
 */
export interface EmailTemplate {
  id: string;
  /** The organization unit this template belongs to. Set at creation and immutable thereafter. */
  ouId: string;
  handle: string;
  displayName: string;
  description?: string;
  design?: TemplateDesign;
  content: EmailTemplateContent;
}

/**
 * A single SMS notification template.
 *
 * @public
 */
export interface SmsTemplate {
  id: string;
  /** The organization unit this template belongs to. Set at creation and immutable thereafter. */
  ouId: string;
  handle: string;
  displayName: string;
  description?: string;
  content: SmsTemplateContent;
}

/**
 * A notification template of either channel.
 *
 * @public
 */
export type NotificationTemplate = EmailTemplate | SmsTemplate;

/**
 * Lightweight template representation returned by the list endpoint.
 *
 * @public
 */
export interface TemplateSummary {
  id: string;
  handle: string;
  displayName: string;
  description?: string;
  self?: string;
}
