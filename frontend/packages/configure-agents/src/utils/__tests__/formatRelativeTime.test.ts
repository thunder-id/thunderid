// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import formatRelativeTime from '../formatRelativeTime';

const NOW = new Date('2026-10-27T12:00:00Z');
const DAY = 24 * 3600 * 1000;

const at = (offsetMs: number): Date => new Date(NOW.getTime() + offsetMs);

describe('formatRelativeTime', () => {
  it('uses the largest whole unit, ahead and behind', () => {
    expect(formatRelativeTime(at(90 * DAY), NOW, 'en')).toBe('in 3 months');
    expect(formatRelativeTime(at(-13 * DAY), NOW, 'en')).toBe('13 days ago');
    expect(formatRelativeTime(at(2 * 365 * DAY), NOW, 'en')).toBe('in 2 years');
    expect(formatRelativeTime(at(5 * 3600 * 1000), NOW, 'en')).toBe('in 5 hours');
    expect(formatRelativeTime(at(-10 * 60 * 1000), NOW, 'en')).toBe('10 minutes ago');
  });

  it('says tomorrow and yesterday rather than in 1 day', () => {
    expect(formatRelativeTime(at(DAY), NOW, 'en')).toBe('tomorrow');
    expect(formatRelativeTime(at(-DAY), NOW, 'en')).toBe('yesterday');
  });

  it('falls to seconds under a minute', () => {
    expect(formatRelativeTime(at(30 * 1000), NOW, 'en')).toBe('in 30 seconds');
  });
});
