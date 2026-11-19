// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {TFunction} from 'i18next';
import {describe, it, expect} from 'vitest';
import {formatDurationShort, toDurationParts, toSeconds} from '../duration';

/**
 * A stand-in for the real catalog, resolving the short unit keys. A key without the settings
 * namespace resolves to itself, so a label that drops the prefix fails its assertion.
 */
const translate = (key: string, options?: Record<string, unknown>): string => {
  const catalog: Record<string, string> = {
    'units.short.second': '{{count}} second',
    'units.short.seconds': '{{count}} seconds',
    'units.short.minute': '{{count}} minute',
    'units.short.minutes': '{{count}} minutes',
    'units.short.hour': '{{count}} hour',
    'units.short.hours': '{{count}} hours',
    'units.short.day': '{{count}} day',
    'units.short.days': '{{count}} days',
  };
  const name: string | undefined = key.startsWith('settings:') ? key.slice('settings:'.length) : undefined;
  return Object.entries(options ?? {}).reduce<string>(
    (text, [variable, value]) => text.replace(`{{${variable}}}`, String(value)),
    (name !== undefined ? catalog[name] : undefined) ?? key,
  );
};
const t = translate as unknown as TFunction;

describe('toDurationParts', () => {
  it('shows a value in the largest unit that divides it exactly', () => {
    expect(toDurationParts(86400)).toEqual({value: 1, unit: 'days'});
    expect(toDurationParts(3600)).toEqual({value: 1, unit: 'hours'});
    expect(toDurationParts(900)).toEqual({value: 15, unit: 'minutes'});
  });

  // Rounding 5400 to "1 hour" would silently halve the setting on the next save.
  it('keeps a value the larger units cannot express exactly', () => {
    expect(toDurationParts(5400)).toEqual({value: 90, unit: 'minutes'});
    expect(toDurationParts(45)).toEqual({value: 45, unit: 'seconds'});
  });

  it('reports zero as zero seconds', () => {
    expect(toDurationParts(0)).toEqual({value: 0, unit: 'seconds'});
  });
});

describe('toSeconds', () => {
  it('converts each unit', () => {
    expect(toSeconds(15, 'minutes')).toBe(900);
    expect(toSeconds(1, 'days')).toBe(86400);
  });

  it('never produces a negative duration', () => {
    expect(toSeconds(-5, 'minutes')).toBe(0);
  });

  it('round-trips every value it can display', () => {
    [60, 300, 900, 3600, 5400, 86400, 172800].forEach((seconds) => {
      const {value, unit} = toDurationParts(seconds);
      expect(toSeconds(value, unit)).toBe(seconds);
    });
  });
});

describe('formatDurationShort', () => {
  it('inflects the units that inflect and leaves the ones that do not', () => {
    expect(formatDurationShort(3600, t)).toBe('1 hour');
    expect(formatDurationShort(7200, t)).toBe('2 hours');
    expect(formatDurationShort(86400, t)).toBe('1 day');
    expect(formatDurationShort(172800, t)).toBe('2 days');
    expect(formatDurationShort(60, t)).toBe('1 minute');
    expect(formatDurationShort(900, t)).toBe('15 minutes');
  });
});
