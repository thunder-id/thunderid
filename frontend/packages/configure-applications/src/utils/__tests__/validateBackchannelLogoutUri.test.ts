// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import validateBackchannelLogoutUri from '../validateBackchannelLogoutUri';

const INVALID_KEY = 'applications:edit.general.backchannelLogoutUri.error.invalid';
const REQUIRES_HTTPS_KEY = 'applications:edit.general.backchannelLogoutUri.error.requiresHttps';

describe('validateBackchannelLogoutUri', () => {
  describe('valid URIs', () => {
    it.each([undefined, '', '   '])('accepts an empty value (%j), which means no notification', (uri) => {
      expect(validateBackchannelLogoutUri(uri, true)).toEqual({valid: true});
    });

    it('accepts an https URI for a public client', () => {
      expect(validateBackchannelLogoutUri('https://rp.example.com/bcl', true)).toEqual({valid: true});
    });

    it('accepts an http URI for a confidential client', () => {
      expect(validateBackchannelLogoutUri('http://rp.internal:8080/bcl', false)).toEqual({valid: true});
    });

    it('accepts a URI with a query string', () => {
      expect(validateBackchannelLogoutUri('https://rp.example.com/bcl?tenant=a', true)).toEqual({valid: true});
    });

    it('leaves private and loopback hosts to the server', () => {
      expect(validateBackchannelLogoutUri('https://localhost/bcl', true)).toEqual({valid: true});
    });
  });

  describe('invalid URIs', () => {
    it.each([
      ['a relative path', '/bcl'],
      ['a non-http scheme', 'ftp://rp.example.com/bcl'],
      ['no host', 'https:///bcl'],
      ['user info', 'https://user:pass@rp.example.com/bcl'],
      ['a fragment', 'https://rp.example.com/bcl#section'],
      ['an empty fragment', 'https://rp.example.com/bcl#'],
      ['a wildcard host', 'https://*.example.com/bcl'],
      ['a wildcard path', 'https://rp.example.com/*'],
    ])('rejects %s', (_label, uri) => {
      const result = validateBackchannelLogoutUri(uri, false);
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe(INVALID_KEY);
      expect(result.errorDefault).toBeTruthy();
    });

    it('rejects an http URI for a public client', () => {
      const result = validateBackchannelLogoutUri('http://rp.example.com/bcl', true);
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe(REQUIRES_HTTPS_KEY);
      expect(result.errorDefault).toBeTruthy();
    });
  });
});
