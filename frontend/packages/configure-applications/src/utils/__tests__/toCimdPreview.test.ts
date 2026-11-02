// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import type {CimdPreviewResponse} from '../../models/cimd';
import type {OAuth2Config} from '../../models/oauth';
import toCimdPreview from '../toCimdPreview';

const response = (config: OAuth2Config, extra: Partial<CimdPreviewResponse> = {}): CimdPreviewResponse => ({
  name: 'Client',
  inboundAuthConfig: [{type: 'oauth2', config}],
  ...extra,
});

describe('toCimdPreview', () => {
  it('maps a public client and keeps the returned OAuth configuration unchanged', () => {
    const config: OAuth2Config = {
      clientId: 'https://client.example.com:8443/client.json',
      clientIdMetadataDocument: true,
      redirectUris: ['http://127.0.0.1/callback', 'http://localhost:3000/callback'],
      grantTypes: ['authorization_code'],
      tokenEndpointAuthMethod: 'none',
    };

    const preview = toCimdPreview(
      response(config, {url: 'https://client.example.com', tosUri: 'https://client.example.com/tos'}),
    );

    expect(preview).toMatchObject({
      clientId: 'https://client.example.com:8443/client.json',
      clientIdHost: 'client.example.com:8443',
      clientName: 'Client',
      clientUri: 'https://client.example.com',
      tosUri: 'https://client.example.com/tos',
      contacts: [],
      tokenEndpointAuthMethod: 'none',
      hasInlineJwks: false,
      loopbackOnly: true,
      grantTypes: ['authorization_code'],
    });
    expect(preview.jwksUri).toBeUndefined();
    expect(preview.oauth2Config).toBe(config);
  });

  it('reads the key source of a private key JWT client', () => {
    const base: OAuth2Config = {
      clientId: 'https://client.example.com/client.json',
      redirectUris: ['https://client.example.com/callback'],
      tokenEndpointAuthMethod: 'private_key_jwt',
    };

    const withUri = toCimdPreview(
      response({...base, certificate: {type: 'JWKS_URI', value: 'https://client.example.com/jwks'}}),
    );
    const inline = toCimdPreview(response({...base, certificate: {type: 'JWKS', value: '{"keys":[]}'}}));

    expect(withUri.jwksUri).toBe('https://client.example.com/jwks');
    expect(withUri.hasInlineJwks).toBe(false);
    expect(withUri.loopbackOnly).toBe(false);
    expect(inline.jwksUri).toBeUndefined();
    expect(inline.hasInlineJwks).toBe(true);
  });
});
