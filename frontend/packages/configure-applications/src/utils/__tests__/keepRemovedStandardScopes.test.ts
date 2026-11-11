// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import type {InboundAuthConfig} from '../../models/inbound-auth';
import keepRemovedStandardScopes from '../keepRemovedStandardScopes';

const oauth = (scopeClaims?: Record<string, string[]>): InboundAuthConfig[] => [
  {type: 'oauth2', config: {clientId: 'client', scopeClaims}} as InboundAuthConfig,
];

const scopeClaimsOf = (configs: InboundAuthConfig[]) => configs[0].config?.scopeClaims;

describe('keepRemovedStandardScopes', () => {
  it('keeps a removed standard scope as an empty list', () => {
    const result = keepRemovedStandardScopes(
      oauth({email: ['email']}),
      oauth({email: ['email'], profile: ['given_name', 'family_name']}),
    );

    expect(scopeClaimsOf(result)).toEqual({email: ['email'], profile: []});
  });

  it('keeps every removed standard scope, including ones that were already empty', () => {
    const result = keepRemovedStandardScopes(oauth({}), oauth({address: [], phone: [], roles: ['roles']}));

    expect(scopeClaimsOf(result)).toEqual({address: [], phone: [], roles: []});
  });

  it('drops a removed custom scope', () => {
    const result = keepRemovedStandardScopes(
      oauth({email: ['email']}),
      oauth({email: ['email'], 'custom:read': ['email']}),
    );

    expect(scopeClaimsOf(result)).toEqual({email: ['email']});
  });

  it('does not add a standard scope the saved mapping never had', () => {
    const result = keepRemovedStandardScopes(oauth({email: ['email']}), oauth({email: ['email']}));

    expect(scopeClaimsOf(result)).toEqual({email: ['email']});
  });

  it('returns the configs unchanged when nothing was removed', () => {
    const edited = oauth({email: ['email'], profile: ['given_name']});

    expect(keepRemovedStandardScopes(edited, oauth({email: ['email']}))).toEqual(edited);
  });
});
