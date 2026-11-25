// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {afterEach, beforeEach, describe, expect, it, vi} from 'vitest';
import formatRelativeTime from '../formatRelativeTime';

const NOW = new Date('2026-11-04T12:00:00Z');

describe('formatRelativeTime', () => {
  beforeEach(() => {
    vi.useFakeTimers();
    vi.setSystemTime(NOW);
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it('describes a past moment in its largest whole unit', () => {
    expect(formatRelativeTime(new Date('2026-11-04T09:00:00Z'), 'en')).toBe('3 hours ago');
  });

  it('describes a future moment', () => {
    expect(formatRelativeTime(new Date('2026-11-06T12:00:00Z'), 'en')).toBe('in 2 days');
  });

  it('uses words for adjacent units', () => {
    expect(formatRelativeTime(new Date('2026-11-03T12:00:00Z'), 'en')).toBe('yesterday');
  });

  it('falls back to seconds under a minute', () => {
    expect(formatRelativeTime(NOW, 'en')).toBe('now');
  });

  it('formats in the given locale', () => {
    expect(formatRelativeTime(new Date('2026-11-04T09:00:00Z'), 'fr')).toBe('il y a 3 heures');
  });
});
