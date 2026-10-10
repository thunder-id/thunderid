// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ValueKind} from './models';

/** A value of a resource that the gateway fills from its own store. */
export interface ValueReference {
  kind: ValueKind;
  name: string;
}

const REFERENCE = /^(var|sec):([A-Za-z_][A-Za-z0-9_]*)$/;

/** Reads a `var:NAME` or `sec:NAME` reference, as a configuration version writes one. */
export function parseReference(value: unknown): ValueReference | undefined {
  if (typeof value !== 'string') return undefined;
  const match = REFERENCE.exec(value);
  if (!match) return undefined;
  return {kind: match[1] === 'sec' ? 'secret' : 'variable', name: match[2]};
}

/** Reads a value held as a JSON list of strings, the form a list's variable takes. */
export function parseListValue(value: string): string[] | undefined {
  if (!value.trim().startsWith('[')) return undefined;
  try {
    const parsed: unknown = JSON.parse(value);
    return Array.isArray(parsed) && parsed.every((item) => typeof item === 'string') ? parsed : undefined;
  } catch {
    return undefined;
  }
}

/** Shows a list's value one item per line, for editing. */
export function toListText(value: string): string {
  return parseListValue(value)?.join('\n') ?? value;
}

/** Writes lines of text as the JSON list a list's variable holds, leaving out blank lines. */
export function fromListText(text: string): string {
  return JSON.stringify(
    text
      .split('\n')
      .map((line: string) => line.trim())
      .filter(Boolean),
  );
}

/** What a secret the gateway holds is shown as: its value is never read back. */
export const MASKED_SECRET = '••••••••';

const isObject = (value: unknown): value is Record<string, unknown> =>
  value !== null && typeof value === 'object' && !Array.isArray(value);

/** Whether a value says nothing worth a row: absent, blank, or an empty list or object. */
export function isEmptyValue(value: unknown): boolean {
  if (value === null || value === undefined) return true;
  if (typeof value === 'string') return value.trim() === '';
  if (Array.isArray(value)) return value.length === 0;
  if (isObject(value)) return Object.values(value).every(isEmptyValue);
  return false;
}
