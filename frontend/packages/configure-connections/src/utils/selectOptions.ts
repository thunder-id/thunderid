// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Ensures a stored select value remains available when it is absent from the current options. */
export function withStoredOption(options: string[], storedValue: string): string[] {
  return storedValue.trim() !== '' && !options.includes(storedValue) ? [storedValue, ...options] : options;
}
