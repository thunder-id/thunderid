// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Formats an API timestamp for display, or a dash when there is none. Go's zero time, which a gateway
 * declared in a file reports as its creation time, is no time either and shows as a dash too.
 */
export default function formatTimestamp(value?: string): string {
  if (!value) return '-';
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  if (date.getUTCFullYear() <= 1) return '-';
  return date.toLocaleString();
}
