// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {shortHash, versionLabel} from '../format';

describe('format', () => {
  it('names a version by its number, else by the first seven characters of its hash', () => {
    expect(shortHash('abcdef0123')).toBe('abcdef0');
    expect(shortHash(undefined)).toBe('');
    expect(versionLabel('abcdef0123', 14)).toBe('v14');
    expect(versionLabel('abcdef0123')).toBe('abcdef0');
  });
});
