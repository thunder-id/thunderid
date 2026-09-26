// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {AllowedOriginTypes, createRow} from '@thunderid/configure-settings';
import {describe, expect, it} from 'vitest';
import hasInvalidCorsRows from '../hasInvalidCorsRows';

const origin = (value: string) => createRow(AllowedOriginTypes.ORIGIN, value);
const regex = (value: string) => createRow(AllowedOriginTypes.REGEX, value);

describe('hasInvalidCorsRows', () => {
  it('is false for an empty list', () => {
    expect(hasInvalidCorsRows([])).toBe(false);
  });

  it('is false for the untouched placeholder row', () => {
    expect(hasInvalidCorsRows([origin('')])).toBe(false);
  });

  it('is false for valid origin and regex rows', () => {
    expect(hasInvalidCorsRows([origin('https://app.example.com'), regex('^https://x\\.io$')])).toBe(false);
  });

  it('is true for an origin row carrying a path', () => {
    expect(hasInvalidCorsRows([origin('https://example.com/path')])).toBe(true);
  });

  it('is true for a regex row that does not compile', () => {
    expect(hasInvalidCorsRows([regex('(bad')])).toBe(true);
  });

  it('is false for an unanchored regex, which only warns', () => {
    expect(hasInvalidCorsRows([regex('acme\\.io')])).toBe(false);
  });
});
