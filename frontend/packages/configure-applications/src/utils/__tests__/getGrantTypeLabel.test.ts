// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {getGrantTypeLabel} from '../getGrantTypeLabel';

const t = (_key: string, fallback: string) => fallback;

describe('getGrantTypeLabel', () => {
  it('returns the friendly CIBA label for the CIBA URN', () => {
    expect(getGrantTypeLabel('urn:openid:params:grant-type:ciba', t)).toBe(
      'CIBA (Client-Initiated Backchannel Authentication)',
    );
  });

  it('returns the raw value unchanged for authorization_code', () => {
    expect(getGrantTypeLabel('authorization_code', t)).toBe('authorization_code');
  });

  it('returns the friendly Token Exchange label for the token-exchange URN', () => {
    expect(getGrantTypeLabel('urn:ietf:params:oauth:grant-type:token-exchange', t)).toBe('Token Exchange');
  });

  it('returns the friendly JWT Bearer label for the jwt-bearer URN', () => {
    expect(getGrantTypeLabel('urn:ietf:params:oauth:grant-type:jwt-bearer', t)).toBe('JWT Bearer');
  });

  it('returns the raw value unchanged for an unknown/arbitrary grant type', () => {
    expect(getGrantTypeLabel('urn:example:custom-grant', t)).toBe('urn:example:custom-grant');
  });
});
