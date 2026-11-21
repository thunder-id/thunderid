// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import formatDuration from '../formatDuration';

describe('formatDuration', () => {
  it('formats days, hours and minutes, dropping the seconds', () => {
    expect(formatDuration(((24 + 2) * 60 + 3) * 60 + 4, 'en-US')).toBe('1d 2h 3m');
  });

  it('leaves out the leading units that are zero but keeps the ones after them', () => {
    expect(formatDuration(2 * 60 * 60 + 5, 'en-US')).toBe('2h 0m');
    expect(formatDuration(5 * 60 + 59, 'en-US')).toBe('5m');
    expect(formatDuration(42, 'en-US')).toBe('0m');
  });
});
