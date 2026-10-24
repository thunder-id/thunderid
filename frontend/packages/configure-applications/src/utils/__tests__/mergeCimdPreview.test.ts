// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {OAuth2GrantTypes, TokenEndpointAuthMethods} from '../../models/oauth';
import type {OAuth2Config} from '../../models/oauth';
import mergeCimdPreview from '../mergeCimdPreview';

const {AUTHORIZATION_CODE, REFRESH_TOKEN, TOKEN_EXCHANGE, JWT_BEARER} = OAuth2GrantTypes;
const jwksUri = {type: 'JWKS_URI', value: 'https://client.example.com/jwks.json'};

const confidentialPreview = (overrides: Partial<OAuth2Config> = {}): OAuth2Config => ({
  clientId: 'https://client.example.com/client.json',
  clientIdMetadataDocument: true,
  redirectUris: ['https://client.example.com/callback'],
  grantTypes: [AUTHORIZATION_CODE, REFRESH_TOKEN],
  responseTypes: ['code'],
  tokenEndpointAuthMethod: TokenEndpointAuthMethods.PRIVATE_KEY_JWT,
  publicClient: false,
  pkceRequired: true,
  requirePushedAuthorizationRequests: false,
  dpopBoundAccessTokens: false,
  certificate: jwksUri,
  ...overrides,
});

const publicPreview = (): OAuth2Config =>
  confidentialPreview({
    tokenEndpointAuthMethod: TokenEndpointAuthMethods.NONE,
    publicClient: true,
    certificate: undefined,
  });

const current = (): OAuth2Config =>
  ({
    ...confidentialPreview(),
    grantTypes: [AUTHORIZATION_CODE, REFRESH_TOKEN, TOKEN_EXCHANGE, JWT_BEARER],
    scopes: ['openid', 'profile'],
    requirePushedAuthorizationRequests: true,
    dpopBoundAccessTokens: true,
    token: {idJag: {enabled: true, allowedAudiences: ['https://as.example.com'], validityPeriod: 300}},
  }) as OAuth2Config;

describe('mergeCimdPreview', () => {
  it('keeps the administrator-added identity assertion grants while the document stays private key JWT', () => {
    const merged = mergeCimdPreview(current(), confidentialPreview());

    expect(merged.grantTypes).toEqual([AUTHORIZATION_CODE, REFRESH_TOKEN, TOKEN_EXCHANGE, JWT_BEARER]);
    expect(merged.token?.idJag?.enabled).toBe(true);
  });

  it('removes the identity assertion grants and turns ID-JAG off when the document becomes public', () => {
    const merged = mergeCimdPreview(current(), publicPreview());

    expect(merged.grantTypes).toEqual([AUTHORIZATION_CODE, REFRESH_TOKEN]);
    expect(merged.tokenEndpointAuthMethod).toBe(TokenEndpointAuthMethods.NONE);
    expect(merged.publicClient).toBe(true);
    expect(merged.certificate).toBeNull();
    expect(merged.token?.idJag).toEqual({
      enabled: false,
      allowedAudiences: ['https://as.example.com'],
      validityPeriod: 300,
    });
  });

  it('takes the grants a newly confidential document declares', () => {
    const publicClient: OAuth2Config = {...publicPreview(), grantTypes: [AUTHORIZATION_CODE]};
    const merged = mergeCimdPreview(publicClient, confidentialPreview());

    expect(merged.grantTypes).toEqual([AUTHORIZATION_CODE, REFRESH_TOKEN]);
    expect(merged.certificate).toEqual(jwksUri);
    expect(merged.publicClient).toBe(false);
  });

  it('keeps a refresh token grant the document no longer declares', () => {
    const merged = mergeCimdPreview(current(), confidentialPreview({grantTypes: [AUTHORIZATION_CODE]}));

    expect(merged.grantTypes).toEqual([AUTHORIZATION_CODE, REFRESH_TOKEN, TOKEN_EXCHANGE, JWT_BEARER]);
  });

  it('adds a grant the document now declares, without listing any grant twice', () => {
    const withoutRefresh: OAuth2Config = {...current(), grantTypes: [AUTHORIZATION_CODE, TOKEN_EXCHANGE, JWT_BEARER]};
    const merged = mergeCimdPreview(withoutRefresh, confidentialPreview());

    expect(merged.grantTypes).toEqual([AUTHORIZATION_CODE, REFRESH_TOKEN, TOKEN_EXCHANGE, JWT_BEARER]);
  });

  it('keeps settings the document does not own', () => {
    const merged = mergeCimdPreview(current(), confidentialPreview({redirectUris: ['https://client.example.com/new']}));

    expect(merged.redirectUris).toEqual(['https://client.example.com/new']);
    expect(merged.scopes).toEqual(['openid', 'profile']);
    expect(merged.requirePushedAuthorizationRequests).toBe(true);
    expect(merged.dpopBoundAccessTokens).toBe(true);
    expect(merged.responseTypes).toEqual(['code']);
  });

  it('leaves the token settings untouched when ID-JAG is off', () => {
    const withoutIdJag: OAuth2Config = {...current(), token: undefined};
    const merged = mergeCimdPreview(withoutIdJag, publicPreview());

    expect(merged.token).toBeUndefined();
  });
});
