// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {CspScopeRow} from '../../models/cspDirectiveRow';
import validateCspDirectiveRows, {isInvalidCspSource} from '../validateCspDirectiveRows';

const row = (override: string[] | undefined, base: string[] = []): CspScopeRow => ({
  directive: 'script-src',
  base,
  override,
  draftId: override ? 'a' : undefined,
});

describe('validateCspDirectiveRows', () => {
  it('ignores rows without an override', () => {
    expect(validateCspDirectiveRows([row(undefined, ["'self'"])])).toEqual({});
  });

  it('accepts a valid override, and an empty directive with no default value', () => {
    expect(validateCspDirectiveRows([row(["'self'", 'data:'], ["'self'"]), row([])])).toEqual({});
  });

  it.each([";'self'", "'self',", 'a b'])('rejects the source %s', (source) => {
    expect(isInvalidCspSource(source)).toBe(true);
    expect(validateCspDirectiveRows([row([source])])).toEqual({a: 'invalidSource'});
  });

  it('rejects removing every source from a directive with a default value', () => {
    expect(validateCspDirectiveRows([row([], ["'self'"])])).toEqual({a: 'noSources'});
  });

  it('rejects emptying a saved directive that has no default value', () => {
    expect(validateCspDirectiveRows([{...row([]), saved: true}])).toEqual({a: 'noSourcesNoDefault'});
  });

  it("rejects 'none' combined with other sources", () => {
    expect(validateCspDirectiveRows([row(["'none'", "'self'"])])).toEqual({a: 'noneWithOthers'});
  });
});
