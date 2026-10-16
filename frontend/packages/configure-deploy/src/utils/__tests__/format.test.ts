// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {formatTimestamp, shortHash, versionLabel} from '../format';

describe('format', () => {
  it('names a version by its number, else by the first seven characters of its hash', () => {
    expect(shortHash('abcdef0123')).toBe('abcdef0');
    expect(shortHash(undefined)).toBe('');
    expect(versionLabel('abcdef0123', 14)).toBe('v14');
    expect(versionLabel('abcdef0123')).toBe('abcdef0');
  });

  it("formats a moment in the reader's locale and leaves what it cannot read as it is", () => {
    expect(formatTimestamp(undefined)).toBe('');
    expect(formatTimestamp('not a date')).toBe('not a date');
    expect(formatTimestamp('2026-10-14T10:00:00Z')).toBe(new Date('2026-10-14T10:00:00Z').toLocaleString());
  });
});
