// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {toDurationParts, toSeconds} from '../duration';

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
