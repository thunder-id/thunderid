// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import {TechnologyApplicationTemplate} from '../../models/application-templates';
import inferApplicationTemplateTechnologyFromConfig from '../inferApplicationTemplateTechnologyFromConfig';
import type {OAuth2Config} from '@thunderid/configure-applications';
import {OAuth2GrantTypes} from '@thunderid/configure-applications';

describe('inferApplicationTemplateTechnologyFromConfig', () => {
  it('returns OTHER for null config', () => {
    const result = inferApplicationTemplateTechnologyFromConfig(null);
    expect(result).toBe(TechnologyApplicationTemplate.OTHER);
  });

  it('returns REACT for public client configurations', () => {
    const config: OAuth2Config = {
      publicClient: true,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid', 'profile'],
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.REACT);
  });

  it('returns NEXTJS for confidential client with authorization code grant', () => {
    const config: OAuth2Config = {
      publicClient: false,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid', 'profile'],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.NEXTJS);
  });

  it('returns OTHER for confidential client without authorization code grant', () => {
    const config: OAuth2Config = {
      publicClient: false,
      grantTypes: [OAuth2GrantTypes.CLIENT_CREDENTIALS],
      responseTypes: [],
      pkceRequired: false,
      scopes: ['openid'],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.OTHER);
  });

  it('returns REACT for public client even with multiple grant types', () => {
    const config: OAuth2Config = {
      publicClient: true,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid', 'profile', 'email'],
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.REACT);
  });

  it('returns NEXTJS for confidential client with authorization code among other grants', () => {
    const config: OAuth2Config = {
      publicClient: false,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE, OAuth2GrantTypes.REFRESH_TOKEN],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid', 'profile', 'email'],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.NEXTJS);
  });

  it('handles config with minimal properties', () => {
    const config: OAuth2Config = {
      publicClient: true,
      grantTypes: [OAuth2GrantTypes.AUTHORIZATION_CODE],
      responseTypes: ['code'],
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.REACT);
  });

  it('handles empty grant types array for public client', () => {
    const config: OAuth2Config = {
      publicClient: true,
      grantTypes: [],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid'],
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.REACT);
  });

  it('handles empty grant types array for confidential client', () => {
    const config: OAuth2Config = {
      publicClient: false,
      grantTypes: [],
      responseTypes: ['code'],
      redirectUris: ['https://localhost:3000/callback'],
      pkceRequired: true,
      scopes: ['openid'],
      tokenEndpointAuthMethod: 'client_secret_basic',
    };

    const result = inferApplicationTemplateTechnologyFromConfig(config);
    expect(result).toBe(TechnologyApplicationTemplate.OTHER);
  });
});
