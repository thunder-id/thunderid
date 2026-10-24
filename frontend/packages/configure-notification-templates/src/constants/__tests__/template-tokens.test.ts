// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {CONTEXT_TOKENS, CONTEXT_TOKEN_KEYS} from '../template-tokens';

describe('CONTEXT_TOKEN_KEYS', () => {
  // The editor preview validates {{ctx(key)}} against this set, so it must stay derived from the catalog
  // the picker offers — never a hand-maintained second list that could drift.
  it('is the ctx key of every CONTEXT_TOKENS entry', () => {
    const expected = CONTEXT_TOKENS.map((token) => /^\{\{ctx\(([A-Za-z0-9_.-]+)\)\}\}$/.exec(token.insert)?.[1]);
    expect(expected.every((key) => key !== undefined)).toBe(true);
    expect(CONTEXT_TOKEN_KEYS).toEqual(new Set(expected));
  });
});
