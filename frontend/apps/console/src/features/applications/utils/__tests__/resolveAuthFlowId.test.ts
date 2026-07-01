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

import {describe, expect, it} from 'vitest';
import {AuthenticatorTypes} from '../../../connections/models/authenticators';
import {AUTH_FLOW_GRAPHS} from '../../models/auth-flow-graphs';
import resolveAuthFlowId from '../resolveAuthFlowId';
import {IdentityProviderTypes, type IdentityProvider} from '@/features/connections/models/identity-provider';

describe('resolveAuthFlowId', () => {
  describe('Constants', () => {
    it('should export AuthenticatorTypes.CREDENTIALS_AUTH', () => {
      expect(AuthenticatorTypes.CREDENTIALS_AUTH).toBe('credentials_auth');
    });
  });

  describe('Single Authentication Method', () => {
    it('should return BASIC flow when only username/password is enabled', () => {
      const result = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC);
      expect(result).toBe('auth_flow_config_basic');
    });

    it('should return GOOGLE flow when only Google is selected', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [googleProvider],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.GOOGLE);
      expect(result).toBe('auth_flow_config_google');
    });

    it('should return GITHUB flow when only GitHub is selected', () => {
      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [githubProvider],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.GITHUB);
      expect(result).toBe('auth_flow_config_github');
    });
  });

  describe('Combined Authentication Methods', () => {
    it('should return BASIC_GOOGLE flow when username/password + Google', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [googleProvider],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC_GOOGLE);
      expect(result).toBe('auth_flow_config_basic_google');
    });

    it('should return BASIC_GOOGLE_GITHUB flow when username/password + Google + GitHub', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [googleProvider, githubProvider],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB);
      expect(result).toBe('auth_flow_config_basic_google_github');
    });

    it('should return BASIC flow when username/password + GitHub (fallback)', () => {
      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [githubProvider],
      });

      // Fallback to BASIC since there's no BASIC_GITHUB flow
      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC);
      expect(result).toBe('auth_flow_config_basic');
    });

    it('should return BASIC_GOOGLE_GITHUB flow when Google + GitHub without username/password (fallback)', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [googleProvider, githubProvider],
      });

      // Fallback to BASIC_GOOGLE_GITHUB since there's no social-only multi-provider flow
      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB);
      expect(result).toBe('auth_flow_config_basic_google_github');
    });
  });

  describe('Multiple Identity Providers', () => {
    it('should handle multiple Google providers (duplicate types)', () => {
      const googleProvider1: IdentityProvider = {
        id: 'google-123',
        name: 'Google 1',
        type: IdentityProviderTypes.GOOGLE,
      };

      const googleProvider2: IdentityProvider = {
        id: 'google-456',
        name: 'Google 2',
        type: IdentityProviderTypes.GOOGLE,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [googleProvider1, googleProvider2],
      });

      // Should still treat as single Google provider
      expect(result).toBe(AUTH_FLOW_GRAPHS.GOOGLE);
    });

    it('should handle providers in different order', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result1 = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [googleProvider, githubProvider],
      });

      const result2 = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [githubProvider, googleProvider],
      });

      // Order shouldn't matter
      expect(result1).toBe(result2);
      expect(result1).toBe(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB);
    });
  });

  describe('Edge Cases', () => {
    it('should return BASIC flow when no authentication method is selected (fallback)', () => {
      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [],
      });

      // Default fallback
      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC);
      expect(result).toBe('auth_flow_config_basic');
    });

    it('should handle empty identity providers array', () => {
      const result = resolveAuthFlowId({
        hasUsernamePassword: true,
        identityProviders: [],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC);
    });

    it('should handle identity provider with different properties', () => {
      const customProvider: IdentityProvider = {
        id: 'google-999',
        name: 'My Custom Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [customProvider],
      });

      // Should only care about the type
      expect(result).toBe(AUTH_FLOW_GRAPHS.GOOGLE);
    });
  });

  describe('Type Extraction Logic', () => {
    it('should correctly extract provider types from identity providers', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [googleProvider, githubProvider],
      });

      // Should detect both types and return appropriate flow
      expect(result).toBe(AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB);
    });

    it('should use type property from IdentityProvider', () => {
      const provider: IdentityProvider = {
        id: 'test-123',
        name: 'Test Provider',
        type: IdentityProviderTypes.GOOGLE,
      };

      const result = resolveAuthFlowId({
        hasUsernamePassword: false,
        identityProviders: [provider],
      });

      expect(result).toBe(AUTH_FLOW_GRAPHS.GOOGLE);
    });
  });

  describe('Comprehensive Scenario Coverage', () => {
    it('should handle all valid two-method combinations', () => {
      const scenarios = [
        {
          config: {hasUsernamePassword: true, identityProviders: []},
          expected: AUTH_FLOW_GRAPHS.BASIC,
        },
        {
          config: {
            hasUsernamePassword: true,
            identityProviders: [{id: '1', name: 'Google', type: IdentityProviderTypes.GOOGLE} as IdentityProvider],
          },
          expected: AUTH_FLOW_GRAPHS.BASIC_GOOGLE,
        },
        {
          config: {
            hasUsernamePassword: true,
            identityProviders: [{id: '2', name: 'GitHub', type: IdentityProviderTypes.GITHUB} as IdentityProvider],
          },
          expected: AUTH_FLOW_GRAPHS.BASIC,
        },
        {
          config: {
            hasUsernamePassword: false,
            identityProviders: [{id: '1', name: 'Google', type: IdentityProviderTypes.GOOGLE} as IdentityProvider],
          },
          expected: AUTH_FLOW_GRAPHS.GOOGLE,
        },
        {
          config: {
            hasUsernamePassword: false,
            identityProviders: [{id: '2', name: 'GitHub', type: IdentityProviderTypes.GITHUB} as IdentityProvider],
          },
          expected: AUTH_FLOW_GRAPHS.GITHUB,
        },
      ];

      scenarios.forEach(({config, expected}) => {
        const result = resolveAuthFlowId(config);
        expect(result).toBe(expected);
      });
    });

    it('should handle all valid three-method combinations', () => {
      const googleProvider: IdentityProvider = {
        id: 'google-123',
        name: 'Google',
        type: IdentityProviderTypes.GOOGLE,
      };

      const githubProvider: IdentityProvider = {
        id: 'github-456',
        name: 'GitHub',
        type: IdentityProviderTypes.GITHUB,
      };

      const scenarios = [
        {
          config: {hasUsernamePassword: true, identityProviders: [googleProvider, githubProvider]},
          expected: AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB,
        },
        {
          config: {hasUsernamePassword: false, identityProviders: [googleProvider, githubProvider]},
          expected: AUTH_FLOW_GRAPHS.BASIC_GOOGLE_GITHUB,
        },
      ];

      scenarios.forEach(({config, expected}) => {
        const result = resolveAuthFlowId(config);
        expect(result).toBe(expected);
      });
    });
  });

  describe('Return Value Verification', () => {
    it('should always return a string', () => {
      const configs = [
        {hasUsernamePassword: true, identityProviders: []},
        {hasUsernamePassword: false, identityProviders: []},
        {
          hasUsernamePassword: true,
          identityProviders: [{id: '1', name: 'Google', type: IdentityProviderTypes.GOOGLE} as IdentityProvider],
        },
      ];

      configs.forEach((config) => {
        const result = resolveAuthFlowId(config);
        expect(typeof result).toBe('string');
        expect(result.length).toBeGreaterThan(0);
      });
    });

    it('should always return a value from AUTH_FLOW_GRAPHS', () => {
      const configs = [
        {hasUsernamePassword: true, identityProviders: []},
        {hasUsernamePassword: false, identityProviders: []},
        {
          hasUsernamePassword: true,
          identityProviders: [{id: '1', name: 'Google', type: IdentityProviderTypes.GOOGLE} as IdentityProvider],
        },
        {
          hasUsernamePassword: false,
          identityProviders: [{id: '1', name: 'Google', type: IdentityProviderTypes.GOOGLE} as IdentityProvider],
        },
        {
          hasUsernamePassword: false,
          identityProviders: [{id: '2', name: 'GitHub', type: IdentityProviderTypes.GITHUB} as IdentityProvider],
        },
      ];

      const validFlowIds = Object.values(AUTH_FLOW_GRAPHS);

      configs.forEach((config) => {
        const result = resolveAuthFlowId(config);
        expect(validFlowIds).toContain(result);
      });
    });

    it('should never return undefined or null', () => {
      const configs = [
        {hasUsernamePassword: true, identityProviders: []},
        {hasUsernamePassword: false, identityProviders: []},
      ];

      configs.forEach((config) => {
        const result = resolveAuthFlowId(config);
        expect(result).not.toBeUndefined();
        expect(result).not.toBeNull();
      });
    });
  });

  describe('Identity Provider Type Constants', () => {
    it('should use IdentityProviderTypes.GOOGLE constant', () => {
      expect(IdentityProviderTypes.GOOGLE).toBe('GOOGLE');
    });

    it('should use IdentityProviderTypes.GITHUB constant', () => {
      expect(IdentityProviderTypes.GITHUB).toBe('GITHUB');
    });
  });
});
