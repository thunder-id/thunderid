// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {AgentInboundAuthConfig} from '../../models/agent';
import isDelegatedAgent, {hasDelegationGrant} from '../isDelegatedAgent';

const oauth = (grantTypes?: string[]): AgentInboundAuthConfig[] => [{type: 'oauth2', config: {grantTypes}}];

describe('isDelegatedAgent', () => {
  it('should be delegated when the agent holds the authorization code grant', () => {
    expect(isDelegatedAgent(oauth(['client_credentials', 'authorization_code']))).toBe(true);
  });

  it('should not be delegated with client credentials only', () => {
    expect(isDelegatedAgent(oauth(['client_credentials']))).toBe(false);
  });

  it('should be delegated with CIBA or token exchange and no authorization code', () => {
    expect(isDelegatedAgent(oauth(['client_credentials', 'urn:openid:params:grant-type:ciba']))).toBe(true);
    expect(isDelegatedAgent(oauth(['client_credentials', 'urn:ietf:params:oauth:grant-type:token-exchange']))).toBe(
      true,
    );
  });

  it('should not be delegated with refresh token alone, which needs a grant to refresh', () => {
    expect(isDelegatedAgent(oauth(['client_credentials', 'refresh_token']))).toBe(false);
  });

  it('should not be delegated without grant types, an OAuth config or any inbound config', () => {
    expect(isDelegatedAgent(oauth())).toBe(false);
    expect(isDelegatedAgent([{type: 'oauth2'}])).toBe(false);
    expect(isDelegatedAgent([])).toBe(false);
    expect(isDelegatedAgent(undefined)).toBe(false);
  });
});

describe('hasDelegationGrant', () => {
  it('should match the grants the server treats as delegation', () => {
    expect(hasDelegationGrant(['authorization_code'])).toBe(true);
    expect(hasDelegationGrant(['urn:openid:params:grant-type:ciba'])).toBe(true);
    expect(hasDelegationGrant(['urn:ietf:params:oauth:grant-type:token-exchange'])).toBe(true);
    expect(hasDelegationGrant(['client_credentials', 'refresh_token'])).toBe(false);
    expect(hasDelegationGrant([])).toBe(false);
    expect(hasDelegationGrant(undefined)).toBe(false);
  });
});
