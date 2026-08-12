// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {AllowedOriginTypes} from '../../models/allowedOriginRow';
import {createRow, isRowEmpty, normalizeRowValue, rowKey, toAllowedOrigins, toRows} from '../allowedOriginRows';

describe('createRowId', () => {
  it('gives every row a distinct id', () => {
    expect(createRow().id).not.toBe(createRow().id);
  });
});

describe('toRows', () => {
  it('takes each row type from the entry shape rather than the text', () => {
    const rows = toRows(['https://app.example.com', {regex: '^https://x\\.io$'}]);
    expect(rows.map(({type, value}) => ({type, value}))).toEqual([
      {type: AllowedOriginTypes.ORIGIN, value: 'https://app.example.com'},
      {type: AllowedOriginTypes.REGEX, value: '^https://x\\.io$'},
    ]);
  });

  it('keeps a regex whose pattern is a valid origin typed as a regex', () => {
    expect(toRows([{regex: 'https://example.com'}])[0].type).toBe(AllowedOriginTypes.REGEX);
  });
});

describe('toAllowedOrigins', () => {
  it('emits the wire shape each row declares', () => {
    expect(
      toAllowedOrigins([
        createRow(AllowedOriginTypes.ORIGIN, 'https://x.com'),
        createRow(AllowedOriginTypes.REGEX, '^https://x\\.com$'),
      ]),
    ).toEqual(['https://x.com', {regex: '^https://x\\.com$'}]);
  });

  it('emits a regex entry for a regex row whose text is itself a valid origin', () => {
    expect(toAllowedOrigins([createRow(AllowedOriginTypes.REGEX, 'https://x.com')])).toEqual([
      {regex: 'https://x.com'},
    ]);
  });

  it('emits a string entry for an origin row with the same text', () => {
    expect(toAllowedOrigins([createRow(AllowedOriginTypes.ORIGIN, 'https://x.com')])).toEqual(['https://x.com']);
  });

  it('drops empty rows', () => {
    expect(toAllowedOrigins([createRow(AllowedOriginTypes.ORIGIN, '  '), createRow(AllowedOriginTypes.REGEX)])).toEqual(
      [],
    );
  });

  it('keeps the "null" literal as a string entry', () => {
    expect(toAllowedOrigins([createRow(AllowedOriginTypes.ORIGIN, 'null')])).toEqual(['null']);
  });
});

describe('normalizeRowValue', () => {
  it('lowercases an origin and strips its trailing slash', () => {
    expect(normalizeRowValue(createRow(AllowedOriginTypes.ORIGIN, ' HTTPS://Example.COM/ '))).toBe(
      'https://example.com',
    );
  });

  it('preserves an explicit default port', () => {
    expect(normalizeRowValue(createRow(AllowedOriginTypes.ORIGIN, 'https://example.com:443'))).toBe(
      'https://example.com:443',
    );
  });

  it('only trims a regex, leaving casing and slashes alone', () => {
    expect(normalizeRowValue(createRow(AllowedOriginTypes.REGEX, '  https://APP.example.com/  '))).toBe(
      'https://APP.example.com/',
    );
  });
});

describe('rowKey', () => {
  it('separates an origin from a regex with the same text', () => {
    expect(rowKey(createRow(AllowedOriginTypes.ORIGIN, 'https://x.com'))).not.toBe(
      rowKey(createRow(AllowedOriginTypes.REGEX, 'https://x.com')),
    );
  });

  it('matches two origins that differ only in case or a trailing slash', () => {
    expect(rowKey(createRow(AllowedOriginTypes.ORIGIN, 'HTTPS://X.com/'))).toBe(
      rowKey(createRow(AllowedOriginTypes.ORIGIN, 'https://x.com')),
    );
  });

  it('separates two regexes that differ only in case', () => {
    expect(rowKey(createRow(AllowedOriginTypes.REGEX, 'https://X.com'))).not.toBe(
      rowKey(createRow(AllowedOriginTypes.REGEX, 'https://x.com')),
    );
  });
});

describe('isRowEmpty', () => {
  it.each([
    ['', true],
    ['   ', true],
    ['https://x.com', false],
  ])('reports %j as %s', (value, expected) => {
    expect(isRowEmpty(createRow(AllowedOriginTypes.ORIGIN, value))).toBe(expected);
  });
});
