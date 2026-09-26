// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import TokenConstants from '../token-constants';

describe('TokenConstants', () => {
  describe('DEFAULT_TOKEN_ATTRIBUTES', () => {
    it('should be defined', () => {
      expect(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES).toBeDefined();
    });

    it('should be an array', () => {
      expect(Array.isArray(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES)).toBe(true);
    });

    it('should contain standard JWT claims', () => {
      const standardClaims = ['aud', 'exp', 'iat', 'iss', 'sub', 'nbf', 'jti'];

      standardClaims.forEach((claim) => {
        expect(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES).toContain(claim);
      });
    });

    it('should contain OAuth2 specific claims', () => {
      const oauth2Claims = ['client_id', 'grant_type', 'scope'];

      oauth2Claims.forEach((claim) => {
        expect(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES).toContain(claim);
      });
    });

    it('should have the expected number of attributes', () => {
      expect(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES).toHaveLength(10);
    });

    it('should not contain duplicate values', () => {
      const unique = new Set(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES);
      expect(unique.size).toBe(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES.length);
    });

    it('should contain all expected attributes in correct order', () => {
      const expectedAttributes = ['aud', 'client_id', 'exp', 'grant_type', 'iat', 'iss', 'jti', 'nbf', 'scope', 'sub'];

      expect(TokenConstants.DEFAULT_TOKEN_ATTRIBUTES).toEqual(expectedAttributes);
    });
  });

  describe('USER_INFO_DEFAULT_ATTRIBUTES', () => {
    it('should be defined', () => {
      expect(TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES).toBeDefined();
    });

    it('should be an array', () => {
      expect(Array.isArray(TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES)).toBe(true);
    });

    it('should contain sub attribute', () => {
      expect(TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES).toContain('sub');
    });

    it('should match expected defaults', () => {
      expect(TokenConstants.USER_INFO_DEFAULT_ATTRIBUTES).toEqual(['sub']);
    });
  });

  describe('ADDITIONAL_USER_ATTRIBUTES', () => {
    it('should be defined', () => {
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toBeDefined();
    });

    it('should be an array', () => {
      expect(Array.isArray(TokenConstants.ADDITIONAL_USER_ATTRIBUTES)).toBe(true);
    });

    it('should contain expected attributes', () => {
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('groups');
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('ouHandle');
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('ouId');
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('ouName');
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('roles');
      expect(TokenConstants.ADDITIONAL_USER_ATTRIBUTES).toContain('userType');
    });
  });
});
