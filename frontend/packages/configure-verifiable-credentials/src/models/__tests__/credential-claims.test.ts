// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {findClaimNameErrors, RESERVED_CLAIM_NAMES, type ClaimRow} from '../credential-claims';

const row = (id: string, name: string): ClaimRow => ({id, name, displayName: ''});

describe('findClaimNameErrors', () => {
  it('returns no errors for distinct names', () => {
    const errors = findClaimNameErrors([row('a', 'given_name'), row('b', 'family_name')]);

    expect(errors).toEqual({});
  });

  it('flags only the repeated row, keeping the first occurrence valid', () => {
    const errors = findClaimNameErrors([row('a', 'given_name'), row('b', 'tier'), row('c', 'given_name')]);

    expect(errors).toEqual({c: 'duplicate'});
  });

  it('compares names case-sensitively, since they become JSON keys', () => {
    const errors = findClaimNameErrors([row('a', 'full_name'), row('b', 'Full_Name')]);

    expect(errors).toEqual({});
  });

  it('ignores surrounding whitespace when comparing', () => {
    const errors = findClaimNameErrors([row('a', 'tier'), row('b', '  tier  ')]);

    expect(errors).toEqual({b: 'duplicate'});
  });

  it('ignores blank rows, which are dropped before the request is built', () => {
    const errors = findClaimNameErrors([row('a', ''), row('b', '   ')]);

    expect(errors).toEqual({});
  });

  it.each(RESERVED_CLAIM_NAMES)('flags the reserved name %s', (name: string) => {
    const errors = findClaimNameErrors([row('a', name)]);

    expect(errors).toEqual({a: 'reserved'});
  });

  it('reports a reserved name as reserved rather than duplicate when repeated', () => {
    const errors = findClaimNameErrors([row('a', 'vct'), row('b', 'vct')]);

    expect(errors).toEqual({a: 'reserved', b: 'reserved'});
  });
});
