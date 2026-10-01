// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** One editable row of a key-value field. */
export interface KeyValuePair {
  name: string;
  value: string;
}

/**
 * Parse the form's serialized representation into editable rows.
 */
export function parseKeyValuePairs(raw: string): KeyValuePair[] {
  return raw
    .split(',')
    .map((segment) => segment.trim())
    .filter((segment) => segment !== '')
    .map((segment) => {
      const separator: number = segment.indexOf(':');
      if (separator === -1) {
        return {name: segment, value: ''};
      }
      return {name: segment.slice(0, separator).trim(), value: segment.slice(separator + 1).trim()};
    });
}

/**
 * Serialize rows for form state. A row is only included when both parts are filled in, so a
 * half-typed row never reaches the API.
 */
export function serializeKeyValuePairs(pairs: KeyValuePair[]): string {
  return pairs
    .filter((pair) => pair.name.trim() !== '' && pair.value.trim() !== '')
    .map((pair) => `${pair.name.trim()}: ${pair.value.trim()}`)
    .join(', ');
}

/**
 * Strip delimiters reserved by the form's serialized representation.
 */
export function sanitizeKeyValuePart(text: string, part: 'name' | 'value'): string {
  const withoutCommas: string = text.replace(/,/g, '');
  return part === 'name' ? withoutCommas.replace(/:/g, '') : withoutCommas;
}
