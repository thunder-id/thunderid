// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import CertificateTypes from '../certificate-types';

describe('CertificateTypes', () => {
  describe('Structure', () => {
    it('should be defined', () => {
      expect(CertificateTypes).toBeDefined();
    });

    it('should be an object', () => {
      expect(typeof CertificateTypes).toBe('object');
    });

    it('should have exactly 3 certificate types', () => {
      expect(Object.keys(CertificateTypes)).toHaveLength(3);
    });
  });

  describe('NONE type', () => {
    it('should have NONE certificate type', () => {
      expect(CertificateTypes).toHaveProperty('NONE');
    });

    it('should have NONE value as "NONE"', () => {
      expect(CertificateTypes.NONE).toBe('NONE');
    });

    it('should have NONE as a string', () => {
      expect(typeof CertificateTypes.NONE).toBe('string');
    });
  });

  describe('JWKS type', () => {
    it('should have JWKS certificate type', () => {
      expect(CertificateTypes).toHaveProperty('JWKS');
    });

    it('should have JWKS value as "JWKS"', () => {
      expect(CertificateTypes.JWKS).toBe('JWKS');
    });

    it('should have JWKS as a string', () => {
      expect(typeof CertificateTypes.JWKS).toBe('string');
    });
  });

  describe('JWKS_URI type', () => {
    it('should have JWKS_URI certificate type', () => {
      expect(CertificateTypes).toHaveProperty('JWKS_URI');
    });

    it('should have JWKS_URI value as "JWKS_URI"', () => {
      expect(CertificateTypes.JWKS_URI).toBe('JWKS_URI');
    });

    it('should have JWKS_URI as a string', () => {
      expect(typeof CertificateTypes.JWKS_URI).toBe('string');
    });
  });

  describe('Type Safety', () => {
    it('should have all expected certificate types', () => {
      const expectedTypes = ['NONE', 'JWKS', 'JWKS_URI'];
      const actualTypes = Object.keys(CertificateTypes);

      expectedTypes.forEach((type) => {
        expect(actualTypes).toContain(type);
      });
    });

    it('should not have duplicate values', () => {
      const values = Object.values(CertificateTypes);
      const uniqueValues = new Set(values);

      expect(uniqueValues.size).toBe(values.length);
    });

    it('should have matching keys and values for each type', () => {
      expect(CertificateTypes.NONE).toBe('NONE');
      expect(CertificateTypes.JWKS).toBe('JWKS');
      expect(CertificateTypes.JWKS_URI).toBe('JWKS_URI');
    });
  });

  describe('Immutability', () => {
    it('should be a const object (TypeScript enforced)', () => {
      // TypeScript ensures this at compile time with 'as const'
      // Runtime check that the object exists and has the expected structure
      expect(CertificateTypes).toBeDefined();
      expect(Object.keys(CertificateTypes)).toHaveLength(3);
    });
  });

  describe('Use Cases', () => {
    it('should support comparison operations', () => {
      let selectedType: string = CertificateTypes.JWKS;
      expect(selectedType === CertificateTypes.JWKS).toBe(true);
      selectedType = CertificateTypes.NONE;
      expect(selectedType === CertificateTypes.NONE).toBe(true);
    });

    it('should work in switch statements', () => {
      const getCertificateLabel = (type: string): string => {
        switch (type) {
          case CertificateTypes.NONE:
            return 'No Certificate';
          case CertificateTypes.JWKS:
            return 'JWKS';
          case CertificateTypes.JWKS_URI:
            return 'JWKS URI';
          default:
            return 'Unknown';
        }
      };

      expect(getCertificateLabel(CertificateTypes.NONE)).toBe('No Certificate');
      expect(getCertificateLabel(CertificateTypes.JWKS)).toBe('JWKS');
      expect(getCertificateLabel(CertificateTypes.JWKS_URI)).toBe('JWKS URI');
    });

    it('should work with array includes checks', () => {
      const validTypes: string[] = [CertificateTypes.JWKS, CertificateTypes.JWKS_URI];

      expect(validTypes.includes(CertificateTypes.JWKS)).toBe(true);
      expect(validTypes.includes(CertificateTypes.NONE)).toBe(false);
    });

    it('should support Object.values() for iteration', () => {
      const allTypes = Object.values(CertificateTypes);

      expect(allTypes).toHaveLength(3);
      expect(allTypes).toContain('NONE');
      expect(allTypes).toContain('JWKS');
      expect(allTypes).toContain('JWKS_URI');
    });
  });
});
