// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ConnectionCategory} from '../models/connection';

/**
 * Ordered list of categories used to render the listing filter chips.
 * Each value maps to the i18n key `connections:categories.<value>`.
 */
export const CONNECTION_CATEGORIES: ConnectionCategory[] = [
  'social-login',
  'enterprise',
  'sms',
  'email',
  'identity-verification',
  'crm',
  'data-store',
  'authorization',
  'trusted-idp',
  'custom',
];
