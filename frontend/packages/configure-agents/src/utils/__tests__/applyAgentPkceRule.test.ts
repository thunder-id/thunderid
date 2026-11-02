// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import applyAgentPkceRule from '../applyAgentPkceRule';

describe('applyAgentPkceRule', () => {
  it('should require PKCE when the next grant types hold authorization code', () => {
    expect(applyAgentPkceRule({grantTypes: ['client_credentials', 'authorization_code']})).toEqual({
      grantTypes: ['client_credentials', 'authorization_code'],
      pkceRequired: true,
    });
  });

  it('should override a PKCE value that the grant change turned off', () => {
    expect(applyAgentPkceRule({grantTypes: ['authorization_code'], pkceRequired: false}).pkceRequired).toBe(true);
  });

  it('should leave the updates unchanged when authorization code is not held', () => {
    const updates = {grantTypes: ['client_credentials'], pkceRequired: false};

    expect(applyAgentPkceRule(updates)).toBe(updates);
  });

  it('should leave the updates unchanged when they do not change the grant types', () => {
    const updates = {backchannelLogoutUri: 'https://agent.example.com/bcl'};

    expect(applyAgentPkceRule(updates)).toBe(updates);
  });

  it('should not mutate the updates it is given', () => {
    const updates = {grantTypes: ['authorization_code']};
    applyAgentPkceRule(updates);

    expect(updates).toEqual({grantTypes: ['authorization_code']});
  });
});
