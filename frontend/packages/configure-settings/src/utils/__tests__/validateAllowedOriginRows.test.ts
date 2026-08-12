// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {AllowedOriginTypes} from '../../models/allowedOriginRow';
import {createRow, rowKey} from '../allowedOriginRows';
import validateAllowedOriginRows from '../validateAllowedOriginRows';

describe('validateAllowedOriginRows', () => {
  it('accepts a valid origin and a valid anchored regex', () => {
    const issues = validateAllowedOriginRows([
      createRow(AllowedOriginTypes.ORIGIN, 'https://app.example.com'),
      createRow(AllowedOriginTypes.REGEX, '^https://x\\.io$'),
    ]);
    expect(issues.errors).toEqual({});
    expect(issues.warnings).toEqual({});
  });

  it.each([
    ['a path', 'https://example.com/path'],
    ['a query string', 'https://example.com?x=1'],
    ['a fragment', 'https://example.com#frag'],
    ['a wildcard host', 'https://*.example.com'],
    ['no scheme', 'example.com'],
  ])('rejects an origin row with %s instead of accepting it as a pattern', (_label, value) => {
    const row = createRow(AllowedOriginTypes.ORIGIN, value);
    expect(validateAllowedOriginRows([row]).errors[row.id]).toBe('invalidOrigin');
  });

  it('rejects a regex row whose pattern does not compile', () => {
    const row = createRow(AllowedOriginTypes.REGEX, '(bad');
    expect(validateAllowedOriginRows([row]).errors[row.id]).toBe('invalidRegex');
  });

  it('accepts a regex row whose pattern would be an invalid origin', () => {
    const row = createRow(AllowedOriginTypes.REGEX, '^https://.*\\.example\\.com/path$');
    expect(validateAllowedOriginRows([row]).errors[row.id]).toBeUndefined();
  });

  it('flags rows that repeat the same entry', () => {
    const rows = [
      createRow(AllowedOriginTypes.ORIGIN, 'https://dup.example.com'),
      createRow(AllowedOriginTypes.ORIGIN, 'HTTPS://Dup.example.com/'),
    ];
    const {errors} = validateAllowedOriginRows(rows);
    expect(errors[rows[0].id]).toBe('duplicate');
    expect(errors[rows[1].id]).toBe('duplicate');
  });

  it('does not flag an origin and a regex that merely share the same text', () => {
    const rows = [
      createRow(AllowedOriginTypes.ORIGIN, 'https://x.com'),
      createRow(AllowedOriginTypes.REGEX, 'https://x.com'),
    ];
    expect(validateAllowedOriginRows(rows).errors).toEqual({});
  });

  it('flags a row that collides with an existing entry', () => {
    const existing = new Set([rowKey(createRow(AllowedOriginTypes.ORIGIN, 'https://console.example.com'))]);
    const row = createRow(AllowedOriginTypes.ORIGIN, 'https://console.example.com');
    expect(validateAllowedOriginRows([row], existing).errors[row.id]).toBe('duplicate');
  });

  it('ignores empty rows entirely', () => {
    const rows = [createRow(AllowedOriginTypes.ORIGIN, '   '), createRow(AllowedOriginTypes.REGEX, '')];
    const issues = validateAllowedOriginRows(rows);
    expect(issues.errors).toEqual({});
    expect(issues.warnings).toEqual({});
  });

  describe('unanchored regex warning', () => {
    it('warns without producing an error', () => {
      const row = createRow(AllowedOriginTypes.REGEX, 'acme\\.io');
      const issues = validateAllowedOriginRows([row]);
      expect(issues.errors[row.id]).toBeUndefined();
      expect(issues.warnings[row.id]).toBe('unanchoredRegex');
    });

    it('is not raised for a pattern that fails to compile', () => {
      const row = createRow(AllowedOriginTypes.REGEX, '(bad');
      expect(validateAllowedOriginRows([row]).warnings[row.id]).toBeUndefined();
    });

    it('is not raised for a literal origin row', () => {
      const row = createRow(AllowedOriginTypes.ORIGIN, 'https://x.com');
      expect(validateAllowedOriginRows([row]).warnings[row.id]).toBeUndefined();
    });
  });
});
