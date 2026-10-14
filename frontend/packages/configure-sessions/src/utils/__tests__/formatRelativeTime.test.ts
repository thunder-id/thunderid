// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import formatRelativeTime from '../formatRelativeTime';

describe('formatRelativeTime', () => {
  const now = new Date('2026-09-30T12:00:00Z');

  it.each([
    ['2026-09-30T11:59:30Z', 'now'],
    ['2026-09-30T11:55:00Z', '5 minutes ago'],
    ['2026-09-30T09:00:00Z', '3 hours ago'],
    ['2026-09-29T12:00:00Z', 'yesterday'],
    ['2026-07-30T12:00:00Z', '2 months ago'],
    ['2026-09-30T12:30:00Z', 'in 30 minutes'],
  ])('formats %s as %s', (value, expected) => {
    expect(formatRelativeTime(value, 'en-US', now)).toBe(expected);
  });
});
