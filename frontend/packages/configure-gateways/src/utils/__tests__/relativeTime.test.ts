// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import relativeTime from '../relativeTime';

const now = Date.parse('2026-11-06T12:00:00Z');

describe('relativeTime', () => {
  it('says how long ago a time was, in the largest unit that fits', () => {
    expect(relativeTime('2026-11-06T10:00:00Z', now, 'en')).toBe('2 hours ago');
    expect(relativeTime('2026-11-05T12:00:00Z', now, 'en')).toBe('yesterday');
    expect(relativeTime('2026-11-06T11:59:30Z', now, 'en')).toBe('now');
  });

  it('says nothing for no time, a time it cannot read, or Go zero time', () => {
    expect(relativeTime(undefined, now)).toBe('');
    expect(relativeTime('not a time', now)).toBe('');
    expect(relativeTime('0001-01-01T00:00:00Z', now)).toBe('');
  });
});
