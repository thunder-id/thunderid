// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Generates a handle from a display name.
 *
 * Lowercases and trims the input, replaces every run of characters outside `[a-z0-9]` with the
 * separator, and removes leading and trailing separators.
 *
 * @example
 * generateHandle('Customer Accounts')      // => 'customer-accounts'
 * generateHandle('  My@Api#V2 ')           // => 'my-api-v2'
 * generateHandle('Payments API', '_')      // => 'payments_api'
 *
 * @param name - The display name to derive the handle from.
 * @param separator - The character that joins the words. Defaults to `-`.
 * @returns The generated handle.
 */
export default function generateHandle(name: string, separator = '-'): string {
  return name
    .toLowerCase()
    .trim()
    .split(/[^a-z0-9]+/)
    .filter(Boolean)
    .join(separator);
}
