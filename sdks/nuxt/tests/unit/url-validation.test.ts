/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {describe, it, expect} from 'vitest';
import {ThunderIDError} from '../../src/runtime/errors/thunderid-error';
import {ErrorCode} from '../../src/runtime/errors/error-codes';
import {validateReturnUrl, safeReturnUrl} from '../../src/runtime/utils/url-validation';

describe('validateReturnUrl', () => {
  describe('valid paths', () => {
    it('accepts a simple relative path', () => {
      expect(validateReturnUrl('/dashboard')).toBe('/dashboard');
    });

    it('accepts a deeply nested relative path', () => {
      expect(validateReturnUrl('/admin/users/1/edit')).toBe('/admin/users/1/edit');
    });

    it('accepts a path with a query string', () => {
      expect(validateReturnUrl('/search?q=foo')).toBe('/search?q=foo');
    });

    it('accepts the root path', () => {
      expect(validateReturnUrl('/')).toBe('/');
    });

    it('trims surrounding whitespace', () => {
      expect(validateReturnUrl('  /dashboard  ')).toBe('/dashboard');
    });
  });

  describe('open redirect attacks — absolute URLs', () => {
    it('rejects an https URL', () => {
      expect(() => validateReturnUrl('https://evil.com')).toThrow(ThunderIDError);
      expect(() => validateReturnUrl('https://evil.com')).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });

    it('rejects an http URL', () => {
      expect(() => validateReturnUrl('http://evil.com/path')).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });
  });

  describe('open redirect attacks — protocol-relative URLs', () => {
    it('rejects a double-slash URL', () => {
      expect(() => validateReturnUrl('//evil.com')).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });

    it('rejects a double-slash URL with a path', () => {
      expect(() => validateReturnUrl('//evil.com/steal?token=abc')).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });
  });

  describe('open redirect attacks — backslash tricks', () => {
    it('rejects a leading backslash after slash', () => {
      expect(() => validateReturnUrl('/\\evil.com')).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });
  });

  describe('invalid types', () => {
    it('rejects null', () => {
      expect(() => validateReturnUrl(null)).toThrow(expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}));
    });

    it('rejects undefined', () => {
      expect(() => validateReturnUrl(undefined)).toThrow(
        expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}),
      );
    });

    it('rejects an empty string', () => {
      expect(() => validateReturnUrl('')).toThrow(expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}));
    });

    it('rejects a whitespace-only string', () => {
      expect(() => validateReturnUrl('   ')).toThrow(expect.objectContaining({code: ErrorCode.OpenRedirectBlocked}));
    });
  });
});

describe('safeReturnUrl', () => {
  it('returns the valid URL on success', () => {
    expect(safeReturnUrl('/profile')).toBe('/profile');
  });

  it('returns the fallback on an invalid URL', () => {
    expect(safeReturnUrl('https://evil.com', '/home')).toBe('/home');
  });

  it('defaults the fallback to "/"', () => {
    expect(safeReturnUrl(null)).toBe('/');
  });
});
