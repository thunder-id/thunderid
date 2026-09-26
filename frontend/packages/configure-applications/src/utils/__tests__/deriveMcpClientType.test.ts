// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import deriveMcpClientType from '../deriveMcpClientType';

describe('deriveMcpClientType', () => {
  it('returns m2m when only client_credentials is granted', () => {
    expect(deriveMcpClientType(['client_credentials'])).toBe('m2m');
  });

  it('returns userDelegated when authorization_code and client_credentials are both granted', () => {
    expect(deriveMcpClientType(['authorization_code', 'client_credentials'])).toBe('userDelegated');
  });

  it('returns userDelegated when only authorization_code and refresh_token are granted', () => {
    expect(deriveMcpClientType(['authorization_code', 'refresh_token'])).toBe('userDelegated');
  });

  it('returns userDelegated when grantTypes is undefined', () => {
    expect(deriveMcpClientType(undefined)).toBe('userDelegated');
  });

  it('returns userDelegated when grantTypes is empty', () => {
    expect(deriveMcpClientType([])).toBe('userDelegated');
  });
});
