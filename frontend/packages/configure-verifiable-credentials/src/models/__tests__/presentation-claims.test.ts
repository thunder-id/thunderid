// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {findDuplicateClaimNames, type ClaimRow} from '../presentation-claims';

const row = (id: string, name: string, requirement: 'mandatory' | 'optional' = 'mandatory'): ClaimRow => ({
  id,
  name,
  requirement,
  values: [],
});

describe('findDuplicateClaimNames', () => {
  it('returns no duplicates for distinct names', () => {
    const duplicates = findDuplicateClaimNames([row('a', 'tier'), row('b', 'full_name')]);

    expect(duplicates).toEqual({});
  });

  it('flags only the repeated row, keeping the first occurrence valid', () => {
    const duplicates = findDuplicateClaimNames([row('a', 'tier'), row('b', 'full_name'), row('c', 'tier')]);

    expect(duplicates).toEqual({c: true});
  });

  it('flags a name used as both mandatory and optional', () => {
    const duplicates = findDuplicateClaimNames([row('a', 'tier', 'mandatory'), row('b', 'tier', 'optional')]);

    expect(duplicates).toEqual({b: true});
  });

  it('ignores surrounding whitespace when comparing', () => {
    const duplicates = findDuplicateClaimNames([row('a', 'tier'), row('b', '  tier  ')]);

    expect(duplicates).toEqual({b: true});
  });

  it('compares names case-sensitively', () => {
    const duplicates = findDuplicateClaimNames([row('a', 'tier'), row('b', 'Tier')]);

    expect(duplicates).toEqual({});
  });

  it('ignores blank rows, which are dropped before the request is built', () => {
    const duplicates = findDuplicateClaimNames([row('a', ''), row('b', '   ')]);

    expect(duplicates).toEqual({});
  });
});
