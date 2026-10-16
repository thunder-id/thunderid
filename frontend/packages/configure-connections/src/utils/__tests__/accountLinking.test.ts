// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {fromAccountLinking, type KeyedLink, newLinkRow, toAccountLinking} from '../accountLinking';

describe('toAccountLinking', () => {
  it('returns undefined when every row is empty', () => {
    const rows: KeyedLink[] = [
      {key: 1, value: ''},
      {key: 2, value: '  '},
    ];
    expect(toAccountLinking(rows)).toBeUndefined();
  });

  it('trims values and drops empty rows', () => {
    const rows: KeyedLink[] = [
      {key: 1, value: ' email '},
      {key: 2, value: ''},
      {key: 3, value: 'username'},
    ];
    expect(toAccountLinking(rows)).toEqual({attributes: ['email', 'username']});
  });
});

describe('fromAccountLinking', () => {
  it('returns a single empty starter row for undefined config', () => {
    const rows = fromAccountLinking(undefined);
    expect(rows).toHaveLength(1);
    expect(rows[0].value).toBe('');
  });

  it('keys every existing attribute, preserving order', () => {
    const rows = fromAccountLinking({attributes: ['email', 'username']});
    expect(rows.map((row) => row.value)).toEqual(['email', 'username']);
    expect(new Set(rows.map((row) => row.key)).size).toBe(rows.length);
  });
});

describe('newLinkRow', () => {
  it('returns a fresh empty row with a unique key', () => {
    const a = newLinkRow();
    const b = newLinkRow();
    expect(a.value).toBe('');
    expect(a.key).not.toBe(b.key);
  });
});
