/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {describe, it, expect, beforeEach, afterEach} from 'vitest';
import generateFallbackRecoveryUrl from '../generateFallbackRecoveryUrl';

describe('generateFallbackRecoveryUrl', () => {
  let originalBaseUrl: string;

  beforeEach(() => {
    originalBaseUrl = import.meta.env.BASE_URL;
  });

  afterEach(() => {
    import.meta.env.BASE_URL = originalBaseUrl;
  });

  describe('base URL handling', () => {
    it('should produce a URL that ends with the recovery path when BASE_URL has a trailing slash', () => {
      import.meta.env.BASE_URL = '/gate/';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).toBe('/gate/recovery');
    });

    it('should produce a URL that ends with the recovery path when BASE_URL has no trailing slash', () => {
      import.meta.env.BASE_URL = '/gate';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).toBe('/gate/recovery');
    });

    it('should handle a root BASE_URL with trailing slash', () => {
      import.meta.env.BASE_URL = '/';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).toBe('/recovery');
    });

    it('should handle an empty BASE_URL', () => {
      import.meta.env.BASE_URL = '';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).toBe('/recovery');
    });
  });

  describe('query string handling', () => {
    it('should append a single query parameter', () => {
      import.meta.env.BASE_URL = '/gate/';
      const params = new URLSearchParams({client_id: 'abc'});
      const result = generateFallbackRecoveryUrl(params);

      expect(result).toBe('/gate/recovery?client_id=abc');
    });

    it('should append multiple query parameters', () => {
      import.meta.env.BASE_URL = '/gate/';
      const params = new URLSearchParams({client_id: 'abc', redirect_uri: 'https://example.com/callback'});
      const result = generateFallbackRecoveryUrl(params);

      // URLSearchParams serialises in insertion order.
      expect(result).toContain('/gate/recovery?');
      expect(result).toContain('client_id=abc');
      expect(result).toContain('redirect_uri=');
    });

    it('should not append a "?" when there are no query params', () => {
      import.meta.env.BASE_URL = '/gate/';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).not.toContain('?');
    });

    it('should strip authId and executionId from the query parameters', () => {
      import.meta.env.BASE_URL = '/gate/';
      const params = new URLSearchParams({
        client_id: 'abc',
        authId: '123',
        executionId: '456',
        redirect_uri: 'https://example.com/callback',
      });
      const result = generateFallbackRecoveryUrl(params);

      expect(result).toContain('/gate/recovery?');
      expect(result).toContain('client_id=abc');
      expect(result).toContain('redirect_uri=');
      expect(result).not.toContain('authId=');
      expect(result).not.toContain('executionId=');
    });

    it('should return the bare path when only authId and executionId are present', () => {
      import.meta.env.BASE_URL = '/gate/';
      const params = new URLSearchParams({authId: '123', executionId: '456'});
      const result = generateFallbackRecoveryUrl(params);

      expect(result).toBe('/gate/recovery');
      expect(result).not.toContain('?');
    });

    it('should return only the recovery path when params are empty', () => {
      import.meta.env.BASE_URL = '/';
      const result = generateFallbackRecoveryUrl(new URLSearchParams());

      expect(result).toBe('/recovery');
    });
  });
});
