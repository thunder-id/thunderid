// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import hasScopeNamesChanged from '../hasScopeNamesChanged';

const oauth = (scopeClaims?: Record<string, string[]>) => [{type: 'oauth2', config: {scopeClaims}}];

describe('hasScopeNamesChanged', () => {
  it('reports a scope added with no attributes', () => {
    expect(hasScopeNamesChanged(oauth({email: ['email'], profile: []}), oauth({email: ['email']}))).toBe(true);
  });

  it('reports a scope with no attributes that was removed', () => {
    expect(hasScopeNamesChanged(oauth({email: ['email']}), oauth({address: [], email: ['email']}))).toBe(true);
  });

  it('ignores changes to the attributes of the same scopes', () => {
    expect(hasScopeNamesChanged(oauth({profile: ['given_name']}), oauth({profile: []}))).toBe(false);
  });

  it('ignores the order of scopes', () => {
    expect(hasScopeNamesChanged(oauth({profile: [], email: []}), oauth({email: [], profile: []}))).toBe(false);
  });

  it('treats a missing mapping as no scopes', () => {
    expect(hasScopeNamesChanged(oauth(), undefined)).toBe(false);
  });
});
