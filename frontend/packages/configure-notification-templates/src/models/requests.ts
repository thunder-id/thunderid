// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {EmailTemplateContent, SmsTemplateContent, TemplateDesign} from './notification-template';

/**
 * Request body to create an email template.
 *
 * @public
 */
export interface CreateEmailTemplateRequest {
  handle: string;
  /**
   * The organization unit this template belongs to. Set at creation and immutable thereafter. A template in a
   * project leaves it out and is placed in the system organization unit.
   */
  ouId?: string;
  /** The project this template belongs to. Required on the control plane. */
  projectId?: string;
  displayName: string;
  description?: string;
  design?: TemplateDesign;
  content: EmailTemplateContent;
}

/**
 * Request body to create an SMS template.
 *
 * @public
 */
export interface CreateSmsTemplateRequest {
  handle: string;
  /**
   * The organization unit this template belongs to. Set at creation and immutable thereafter. A template in a
   * project leaves it out and is placed in the system organization unit.
   */
  ouId?: string;
  /** The project this template belongs to. Required on the control plane. */
  projectId?: string;
  displayName: string;
  description?: string;
  content: SmsTemplateContent;
}

/**
 * Request body to create a template of either channel.
 *
 * @public
 */
export type CreateTemplateRequest = CreateEmailTemplateRequest | CreateSmsTemplateRequest;

/**
 * Request body to update an email template. The handle is immutable, so it is omitted.
 *
 * @public
 */
export interface UpdateEmailTemplateRequest {
  displayName: string;
  description?: string;
  design?: TemplateDesign;
  content: EmailTemplateContent;
}

/**
 * Request body to update an SMS template. The handle is immutable, so it is omitted.
 *
 * @public
 */
export interface UpdateSmsTemplateRequest {
  displayName: string;
  description?: string;
  content: SmsTemplateContent;
}

/**
 * Request body to update a template of either channel.
 *
 * @public
 */
export type UpdateTemplateRequest = UpdateEmailTemplateRequest | UpdateSmsTemplateRequest;
