// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import resolveApplicationType, {isM2MApplication} from '../resolveApplicationType';
import type {OAuth2Config} from '@thunderid/configure-applications';

const makeConfig = (overrides: Partial<OAuth2Config>): OAuth2Config => ({
  publicClient: false,
  grantTypes: [],
  responseTypes: [],
  redirectUris: [],
  pkceRequired: false,
  scopes: [],
  ...overrides,
});

describe('resolveApplicationType', () => {
  it('returns the explicit type when it is a known non-custom type', () => {
    expect(resolveApplicationType('m2m')).toBe('m2m');
    expect(resolveApplicationType('mobile')).toBe('mobile');
    expect(resolveApplicationType('browser', makeConfig({grantTypes: ['client_credentials']}))).toBe('browser');
    expect(resolveApplicationType('mcp', makeConfig({grantTypes: ['client_credentials']}))).toBe('mcp');
  });

  it('falls back to config inference for legacy (undefined) type', () => {
    expect(resolveApplicationType(undefined, makeConfig({grantTypes: ['client_credentials']}))).toBe('m2m');
    expect(
      resolveApplicationType(undefined, makeConfig({publicClient: true, grantTypes: ['authorization_code']})),
    ).toBe('browser');
    expect(resolveApplicationType(undefined, makeConfig({grantTypes: ['authorization_code', 'refresh_token']}))).toBe(
      'fullstack',
    );
  });

  it('falls back to config inference for custom type', () => {
    expect(resolveApplicationType('custom', makeConfig({grantTypes: ['client_credentials']}))).toBe('m2m');
  });

  it('returns custom when neither type nor a recognizable config is present', () => {
    expect(resolveApplicationType(undefined, makeConfig({grantTypes: []}))).toBe('custom');
    expect(resolveApplicationType('custom', makeConfig({grantTypes: []}))).toBe('custom');
  });
});

describe('isM2MApplication', () => {
  it('detects m2m from the explicit type', () => {
    expect(isM2MApplication('m2m')).toBe(true);
    expect(isM2MApplication('browser')).toBe(false);
  });

  it('detects m2m from config shape for legacy/custom apps', () => {
    expect(isM2MApplication(undefined, makeConfig({grantTypes: ['client_credentials']}))).toBe(true);
    expect(isM2MApplication('custom', makeConfig({grantTypes: ['authorization_code']}))).toBe(false);
  });
});
