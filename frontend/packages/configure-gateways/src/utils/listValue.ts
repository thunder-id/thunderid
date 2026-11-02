// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/** Reads a value held as a JSON list of strings, the form a list's variable takes. */
export function parseListValue(value: string): string[] | undefined {
  if (!value.trim().startsWith('[')) return undefined;
  try {
    const parsed: unknown = JSON.parse(value);
    return Array.isArray(parsed) && parsed.every((item: unknown): item is string => typeof item === 'string')
      ? parsed
      : undefined;
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
