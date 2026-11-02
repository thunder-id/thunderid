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

  // The account-lock email templates fill these keys, so the editor must not flag them as unknown.
  it('offers the account-lock email keys', () => {
    expect(CONTEXT_TOKEN_KEYS.has('lockedAuthenticationMethod')).toBe(true);
    expect(CONTEXT_TOKEN_KEYS.has('lockDuration')).toBe(true);
    expect(CONTEXT_TOKEN_KEYS.has('unlockTime')).toBe(true);
  });

  it('gives every lock email token a context label key', () => {
    const lockTokens = CONTEXT_TOKENS.filter((token) =>
      ['{{ctx(lockedAuthenticationMethod)}}', '{{ctx(lockDuration)}}', '{{ctx(unlockTime)}}'].includes(token.insert),
    );
    expect(lockTokens.map((token) => token.labelKey)).toEqual([
      'editor.tokens.labels.context.lockedAuthenticationMethod',
      'editor.tokens.labels.context.lockDuration',
      'editor.tokens.labels.context.unlockTime',
    ]);
  });
});
