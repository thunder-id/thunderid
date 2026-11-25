// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** One editable row of a key-value field. */
export interface KeyValuePair {
  name: string;
  value: string;
}

/**
 * How a key-value field holds its rows in the form state: the "Key: value, ..." text its API also
 * takes, or a JSON array for an API that takes a list, so values keep their commas.
 */
export type KeyValueFormat = 'string' | 'pairs';

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
 * Strip characters the wire format cannot represent: commas separate pairs in the string format,
 * so they are removed from both parts there, and the first colon separates name from value, so it
 * is removed from names.
 */
export function sanitizeKeyValuePart(text: string, part: 'name' | 'value', format: KeyValueFormat = 'string'): string {
  if (part === 'value') {
    return format === 'pairs' ? text : text.replace(/,/g, '');
  }
  return text.replace(/[,:]/g, '');
}

/** Read rows from a form value held in the given format. */
export function decodeKeyValuePairs(raw: string, format: KeyValueFormat = 'string'): KeyValuePair[] {
  if (format === 'string') {
    return parseKeyValuePairs(raw);
  }
  if (raw.trim() === '') {
    return [];
  }
  try {
    const parsed: unknown = JSON.parse(raw);
    return Array.isArray(parsed) ? (parsed as KeyValuePair[]) : [];
  } catch {
    return [];
  }
}

/**
 * Write rows to a form value in the given format. The string format leaves out rows that are not
 * filled in. The pairs format keeps a half-filled row, so clearing a stored value fails validation
 * instead of silently deleting the row, and leaves out only blank rows.
 */
export function encodeKeyValuePairs(pairs: KeyValuePair[], format: KeyValueFormat = 'string'): string {
  if (format === 'string') {
    return serializeKeyValuePairs(pairs);
  }
  const kept: KeyValuePair[] = pairs
    .map((pair) => ({name: pair.name.trim(), value: pair.value.trim()}))
    .filter((pair) => pair.name !== '' || pair.value !== '');
  return kept.length > 0 ? JSON.stringify(kept) : '';
}

/** Whether a row has both its name and its value. */
export function isCompleteKeyValuePair(pair: KeyValuePair): boolean {
  return pair.name.trim() !== '' && pair.value.trim() !== '';
}
