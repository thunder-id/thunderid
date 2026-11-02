// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {
  decodeKeyValuePairs,
  encodeKeyValuePairs,
  parseKeyValuePairs,
  sanitizeKeyValuePart,
  serializeKeyValuePairs,
} from '../keyValuePairs';

describe('parseKeyValuePairs', () => {
  it('splits comma-separated pairs and trims both parts', () => {
    expect(parseKeyValuePairs('X-API-Key: abc123,  Accept : application/json ')).toEqual([
      {name: 'X-API-Key', value: 'abc123'},
      {name: 'Accept', value: 'application/json'},
    ]);
  });

  it('returns no rows for an empty or blank value', () => {
    expect(parseKeyValuePairs('')).toEqual([]);
    expect(parseKeyValuePairs('  ,  ')).toEqual([]);
  });

  it('keeps only the first colon as the separator, so values may contain colons', () => {
    expect(parseKeyValuePairs('Authorization: Bearer a:b:c')).toEqual([{name: 'Authorization', value: 'Bearer a:b:c'}]);
  });

  it('keeps a segment with no colon as a name so it stays visible and fixable', () => {
    expect(parseKeyValuePairs('BrokenHeader')).toEqual([{name: 'BrokenHeader', value: ''}]);
  });
});

describe('serializeKeyValuePairs', () => {
  it('joins pairs in the format the backend parses', () => {
    expect(
      serializeKeyValuePairs([
        {name: 'X-API-Key', value: 'abc123'},
        {name: 'Accept', value: 'application/json'},
      ]),
    ).toBe('X-API-Key: abc123, Accept: application/json');
  });

  it('drops half-filled rows so they never reach the API', () => {
    expect(
      serializeKeyValuePairs([
        {name: '', value: 'orphan'},
        {name: '  ', value: ''},
        {name: 'X-Real', value: 'v'},
      ]),
    ).toBe('X-Real: v');
  });

  it('drops a named row whose value is empty, name included', () => {
    expect(serializeKeyValuePairs([{name: 'X-Flag', value: ''}])).toBe('');
    expect(
      serializeKeyValuePairs([
        {name: 'Content-Type', value: 'application/json'},
        {name: 'Accept', value: '   '},
      ]),
    ).toBe('Content-Type: application/json');
  });

  it('round-trips a parsed value unchanged', () => {
    const wire = 'X-API-Key: abc123, Accept: application/json';
    expect(serializeKeyValuePairs(parseKeyValuePairs(wire))).toBe(wire);
  });
});

describe('sanitizeKeyValuePart', () => {
  it('strips commas from both parts, since commas separate pairs on the wire', () => {
    expect(sanitizeKeyValuePart('a,b', 'name')).toBe('ab');
    expect(sanitizeKeyValuePart('text/html, application/json', 'value')).toBe('text/html application/json');
  });

  it('strips colons from names only', () => {
    expect(sanitizeKeyValuePart('X-A:B', 'name')).toBe('X-AB');
    expect(sanitizeKeyValuePart('Bearer a:b', 'value')).toBe('Bearer a:b');
  });
});

describe('pairs format', () => {
  it('round-trips values that contain commas and colons', () => {
    const pairs = [
      {name: 'Accept', value: 'application/json, text/plain'},
      {name: 'X-List', value: 'a, b: c'},
    ];

    expect(decodeKeyValuePairs(encodeKeyValuePairs(pairs, 'pairs'), 'pairs')).toEqual(pairs);
  });

  it('keeps a half-filled row so it is validated rather than silently dropped', () => {
    expect(decodeKeyValuePairs(encodeKeyValuePairs([{name: ' X-Tenant ', value: ''}], 'pairs'), 'pairs')).toEqual([
      {name: 'X-Tenant', value: ''},
    ]);
    expect(decodeKeyValuePairs(encodeKeyValuePairs([{name: '', value: 'acme'}], 'pairs'), 'pairs')).toEqual([
      {name: '', value: 'acme'},
    ]);
  });

  it('drops only blank rows and encodes no rows as empty', () => {
    expect(encodeKeyValuePairs([{name: '  ', value: ' '}], 'pairs')).toBe('');
    expect(decodeKeyValuePairs('', 'pairs')).toEqual([]);
  });

  it('keeps commas in values', () => {
    expect(sanitizeKeyValuePart('a, b', 'value', 'pairs')).toBe('a, b');
    expect(sanitizeKeyValuePart('X-A,B:C', 'name', 'pairs')).toBe('X-ABC');
  });
});
