// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {ConnectionTypes, type ConnectionResponse} from '../../models/connection';
import type {TrustedIssuer} from '../../models/trusted-issuer';
import mapConnectionToTrustedIssuer from '../mapConnectionToTrustedIssuer';

const BASE_CONNECTION: ConnectionResponse = {
  id: 'ti-1',
  type: ConnectionTypes.OIDC,
  name: 'Acme Okta',
  ouId: 'ou-1',
  clientId: '',
  redirectUri: '',
  authorizationEndpoint: '',
  tokenEndpoint: '',
  issuer: 'https://acme.okta.com',
  jwksEndpoint: 'https://acme.okta.com/keys',
  idJagEnabled: true,
};

describe('mapConnectionToTrustedIssuer', () => {
  it('should map the core trusted-issuer fields', () => {
    const result: TrustedIssuer = mapConnectionToTrustedIssuer(BASE_CONNECTION);

    expect(result).toEqual(
      expect.objectContaining({
        id: 'ti-1',
        name: 'Acme Okta',
        ouId: 'ou-1',
        issuer: 'https://acme.okta.com',
        jwksEndpoint: 'https://acme.okta.com/keys',
        idJagEnabled: true,
      }),
    );
  });
});
