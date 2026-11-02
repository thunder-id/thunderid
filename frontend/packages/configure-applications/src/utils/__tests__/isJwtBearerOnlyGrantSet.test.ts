// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {OAuth2GrantTypes} from '../../models/oauth';
import isJwtBearerOnlyGrantSet from '../isJwtBearerOnlyGrantSet';

describe('isJwtBearerOnlyGrantSet', () => {
  it('should be true when the JWT bearer grant is the only grant', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.JWT_BEARER])).toBe(true);
  });

  it('should be true when the JWT bearer grant is paired only with refresh_token', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.JWT_BEARER, OAuth2GrantTypes.REFRESH_TOKEN])).toBe(true);
  });

  it('should be false when the JWT bearer grant is paired with authorization_code', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.JWT_BEARER, OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(false);
  });

  it('should be false when the JWT bearer grant is paired with client_credentials', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.JWT_BEARER, OAuth2GrantTypes.CLIENT_CREDENTIALS])).toBe(false);
  });

  it('should be false when the JWT bearer grant is paired with token exchange', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.JWT_BEARER, OAuth2GrantTypes.TOKEN_EXCHANGE])).toBe(false);
  });

  it('should be false when the JWT bearer grant is absent', () => {
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.REFRESH_TOKEN])).toBe(false);
    expect(isJwtBearerOnlyGrantSet([OAuth2GrantTypes.AUTHORIZATION_CODE])).toBe(false);
  });

  it('should be false for an empty or missing grant list', () => {
    expect(isJwtBearerOnlyGrantSet([])).toBe(false);
    expect(isJwtBearerOnlyGrantSet(undefined)).toBe(false);
  });
});
