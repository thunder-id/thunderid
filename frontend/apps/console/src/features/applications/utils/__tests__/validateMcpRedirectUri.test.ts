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

import {describe, expect, it} from 'vitest';
import validateMcpRedirectUri from '../validateMcpRedirectUri';

describe('validateMcpRedirectUri', () => {
  describe('valid URIs', () => {
    it('accepts a plain loopback localhost URI', () => {
      expect(validateMcpRedirectUri('http://localhost/cb')).toEqual({valid: true});
    });

    it('accepts a loopback localhost URI with an explicit port', () => {
      expect(validateMcpRedirectUri('http://localhost:3000/cb')).toEqual({valid: true});
    });

    it('accepts a loopback 127.0.0.1 URI with an explicit port', () => {
      expect(validateMcpRedirectUri('http://127.0.0.1:8080/callback')).toEqual({valid: true});
    });

    it('accepts an HTTPS URI', () => {
      expect(validateMcpRedirectUri('https://agent.example.com/oauth/cb')).toEqual({valid: true});
    });

    it('accepts a loopback IPv6 [::1] URI', () => {
      expect(validateMcpRedirectUri('http://[::1]/callback')).toEqual({valid: true});
    });

    it('accepts a loopback IPv6 [::1] URI with an explicit port', () => {
      expect(validateMcpRedirectUri('http://[::1]:8080/callback')).toEqual({valid: true});
    });
  });

  describe('invalid URIs', () => {
    it('rejects a wildcard port on a loopback host', () => {
      const result = validateMcpRedirectUri('http://127.0.0.1:*/callback');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects a wildcard host on HTTPS', () => {
      const result = validateMcpRedirectUri('https://*/callback');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects a wildcard subdomain on HTTPS', () => {
      const result = validateMcpRedirectUri('https://*.example.com/callback');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects a wildcard subdomain that could match an attacker-controlled host', () => {
      const result = validateMcpRedirectUri('https://*.evil.com/callback');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects a custom scheme', () => {
      const result = validateMcpRedirectUri('myapp://callback');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects plain http on a non-loopback host', () => {
      const result = validateMcpRedirectUri('http://example.com/cb');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects the ftp scheme', () => {
      const result = validateMcpRedirectUri('ftp://x');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects an empty string', () => {
      const result = validateMcpRedirectUri('');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.empty');
    });

    it('rejects a URI with no scheme', () => {
      const result = validateMcpRedirectUri('mcp.example.com/cb');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.invalid');
    });

    it('rejects a whitespace-only string as empty', () => {
      const result = validateMcpRedirectUri('   ');
      expect(result.valid).toBe(false);
      expect(result.errorKey).toBe('applications:onboarding.mcp.connection.redirectUris.error.empty');
    });
  });
});
