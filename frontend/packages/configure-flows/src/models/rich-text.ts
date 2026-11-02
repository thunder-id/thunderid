// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Update types for the rich text editor.
 */
export const UPDATE_TYPES = {
  INTERNAL: 'internal',
  EXTERNAL: 'external',
  NONE: 'none',
} as const;

/**
 * Type representing the possible update types.
 */
export type UpdateType = (typeof UPDATE_TYPES)[keyof typeof UPDATE_TYPES];
