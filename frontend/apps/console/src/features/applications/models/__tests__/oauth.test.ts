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
import {OAuth2GrantTypes, OAuth2ResponseTypes, TokenEndpointAuthMethods} from '../oauth';
import type {OAuth2GrantType, OAuth2ResponseType, TokenEndpointAuthMethod, ScopeClaims} from '../oauth';

describe('OAuth Models', () => {
  describe('OAuth2GrantTypes', () => {
    it('should have AUTHORIZATION_CODE grant type', () => {
      expect(OAuth2GrantTypes.AUTHORIZATION_CODE).toBe('authorization_code');
    });

    it('should have REFRESH_TOKEN grant type', () => {
      expect(OAuth2GrantTypes.REFRESH_TOKEN).toBe('refresh_token');
    });

    it('should have CLIENT_CREDENTIALS grant type', () => {
      expect(OAuth2GrantTypes.CLIENT_CREDENTIALS).toBe('client_credentials');
    });

    it('should have PASSWORD grant type', () => {
      expect(OAuth2GrantTypes.PASSWORD).toBe('password');
    });

    it('should have IMPLICIT grant type', () => {
      expect(OAuth2GrantTypes.IMPLICIT).toBe('implicit');
    });

    it('should have CIBA grant type', () => {
      expect(OAuth2GrantTypes.CIBA).toBe('urn:openid:params:grant-type:ciba');
    });

    it('should have all expected grant types', () => {
      const expectedKeys = [
        'AUTHORIZATION_CODE',
        'REFRESH_TOKEN',
        'CLIENT_CREDENTIALS',
        'PASSWORD',
        'IMPLICIT',
        'CIBA',
      ];

      expect(Object.keys(OAuth2GrantTypes)).toEqual(expectedKeys);
    });
  });

  describe('OAuth2ResponseTypes', () => {
    it('should have CODE response type', () => {
      expect(OAuth2ResponseTypes.CODE).toBe('code');
    });

    it('should have TOKEN response type', () => {
      expect(OAuth2ResponseTypes.TOKEN).toBe('token');
    });

    it('should have ID_TOKEN response type', () => {
      expect(OAuth2ResponseTypes.ID_TOKEN).toBe('id_token');
    });

    it('should have CODE_TOKEN response type', () => {
      expect(OAuth2ResponseTypes.CODE_TOKEN).toBe('code token');
    });

    it('should have CODE_ID_TOKEN response type', () => {
      expect(OAuth2ResponseTypes.CODE_ID_TOKEN).toBe('code id_token');
    });

    it('should have TOKEN_ID_TOKEN response type', () => {
      expect(OAuth2ResponseTypes.TOKEN_ID_TOKEN).toBe('token id_token');
    });

    it('should have all expected response types', () => {
      const expectedKeys = ['CODE', 'TOKEN', 'ID_TOKEN', 'CODE_TOKEN', 'CODE_ID_TOKEN', 'TOKEN_ID_TOKEN'];

      expect(Object.keys(OAuth2ResponseTypes)).toEqual(expectedKeys);
    });
  });

  describe('Type Validation', () => {
    it('should accept valid OAuth2GrantType values', () => {
      const validGrantTypes: OAuth2GrantType[] = [
        'authorization_code',
        'refresh_token',
        'client_credentials',
        'password',
        'implicit',
        'urn:openid:params:grant-type:ciba',
      ];

      validGrantTypes.forEach((type) => {
        expect(typeof type).toBe('string');
      });
    });

    it('should accept valid OAuth2ResponseType values', () => {
      const validResponseTypes: OAuth2ResponseType[] = [
        'code',
        'token',
        'id_token',
        'code token',
        'code id_token',
        'token id_token',
      ];

      validResponseTypes.forEach((type) => {
        expect(typeof type).toBe('string');
      });
    });

    it('should accept valid TokenEndpointAuthMethod values', () => {
      const validAuthMethods: TokenEndpointAuthMethod[] = [
        'client_secret_basic',
        'client_secret_post',
        'client_secret_jwt',
        'private_key_jwt',
        'none',
      ];

      validAuthMethods.forEach((method) => {
        expect(typeof method).toBe('string');
      });
    });
  });

  describe('TokenEndpointAuthMethods', () => {
    it('should have CLIENT_SECRET_BASIC auth method', () => {
      expect(TokenEndpointAuthMethods.CLIENT_SECRET_BASIC).toBe('client_secret_basic');
    });

    it('should have CLIENT_SECRET_POST auth method', () => {
      expect(TokenEndpointAuthMethods.CLIENT_SECRET_POST).toBe('client_secret_post');
    });

    it('should have CLIENT_SECRET_JWT auth method', () => {
      expect(TokenEndpointAuthMethods.CLIENT_SECRET_JWT).toBe('client_secret_jwt');
    });

    it('should have PRIVATE_KEY_JWT auth method', () => {
      expect(TokenEndpointAuthMethods.PRIVATE_KEY_JWT).toBe('private_key_jwt');
    });

    it('should have NONE auth method', () => {
      expect(TokenEndpointAuthMethods.NONE).toBe('none');
    });

    it('should have all expected auth method keys', () => {
      const expectedKeys = [
        'CLIENT_SECRET_BASIC',
        'CLIENT_SECRET_POST',
        'CLIENT_SECRET_JWT',
        'PRIVATE_KEY_JWT',
        'NONE',
      ];

      expect(Object.keys(TokenEndpointAuthMethods)).toEqual(expectedKeys);
    });
  });

  describe('ScopeClaims', () => {
    it('should accept standard OIDC scope keys as arrays of strings', () => {
      const scopeClaims: ScopeClaims = {
        profile: ['given_name', 'family_name', 'picture'],
        email: ['email', 'email_verified'],
        phone: ['phone_number', 'phone_number_verified'],
        group: ['groups'],
      };

      expect(Array.isArray(scopeClaims.profile)).toBe(true);
      expect(Array.isArray(scopeClaims.email)).toBe(true);
      expect(Array.isArray(scopeClaims.phone)).toBe(true);
      expect(Array.isArray(scopeClaims.group)).toBe(true);

      scopeClaims.profile?.forEach((claim) => expect(typeof claim).toBe('string'));
      scopeClaims.email?.forEach((claim) => expect(typeof claim).toBe('string'));
      scopeClaims.phone?.forEach((claim) => expect(typeof claim).toBe('string'));
      scopeClaims.group?.forEach((claim) => expect(typeof claim).toBe('string'));
    });
  });
});
