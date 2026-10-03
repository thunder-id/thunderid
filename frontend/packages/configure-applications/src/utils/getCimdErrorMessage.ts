// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import getApplicationErrorMessage from './getApplicationErrorMessage';

/**
 * Resolves a failed metadata document preview to the message shown beside it, through the
 * `errors.CIMD-*` catalog entries in the applications namespace.
 */
export default function getCimdErrorMessage(
  error: Error,
  t: (key: string, options?: Record<string, unknown>) => string,
): string {
  return getApplicationErrorMessage(
    error,
    (key, options) => t(key.includes(':') ? key : `applications:${key}`, options),
    'cimd.errors.generic',
    "This metadata document can't be used.",
  );
}
