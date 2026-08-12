// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import isRegexAnchored from '../isRegexAnchored';

describe('isRegexAnchored', () => {
  // Mirrors the backend's own anchor check, which decides whether it logs a warning for an entry.
  it.each([
    ['^https://x\\.io$', true],
    ['\\Ahttps://x\\.io\\z', true],
    ['^https://x\\.io\\z', true],
    ['^https://x\\.io', false],
    ['https://x\\.io$', false],
    ['https://x\\.io', false],
    ['', false],
  ])('reports %j as %s', (pattern, expected) => {
    expect(isRegexAnchored(pattern)).toBe(expected);
  });

  it('ignores surrounding whitespace', () => {
    expect(isRegexAnchored('  ^https://x\\.io$  ')).toBe(true);
  });

  // An escaped anchor is literal text, so the pattern still searches rather than matching in full.
  it.each([
    ['^https://x\\.io\\$', false],
    ['^https://x\\.io\\\\$', true],
    ['^https://x\\.io\\\\\\$', false],
    ['\\Ahttps://x\\.io\\\\z', false],
    ['^\\$', false],
    ['^\\\\$', true],
  ])('reports %j as %s, reading the backslashes before the anchor', (pattern, expected) => {
    expect(isRegexAnchored(pattern)).toBe(expected);
  });
});
