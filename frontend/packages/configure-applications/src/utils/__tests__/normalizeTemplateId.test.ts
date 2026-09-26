// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it} from 'vitest';
import normalizeTemplateId from '../normalizeTemplateId';

describe('normalizeTemplateId', () => {
  describe('Basic Functionality', () => {
    it('should remove -embedded suffix from template ID', () => {
      const result = normalizeTemplateId('react-embedded');

      expect(result).toBe('react');
    });

    it('should return the same template ID if no -embedded suffix', () => {
      const result = normalizeTemplateId('react');

      expect(result).toBe('react');
    });

    it('should handle various template IDs with -embedded suffix', () => {
      expect(normalizeTemplateId('nextjs-embedded')).toBe('nextjs');
      expect(normalizeTemplateId('angular-embedded')).toBe('angular');
      expect(normalizeTemplateId('vue-embedded')).toBe('vue');
    });

    it('should handle various template IDs without -embedded suffix', () => {
      expect(normalizeTemplateId('nextjs')).toBe('nextjs');
      expect(normalizeTemplateId('angular')).toBe('angular');
      expect(normalizeTemplateId('browser')).toBe('browser');
    });
  });

  describe('Edge Cases', () => {
    it('should return undefined for undefined input', () => {
      const result = normalizeTemplateId(undefined);

      expect(result).toBeUndefined();
    });

    it('should return empty string for empty string input', () => {
      const result = normalizeTemplateId('');

      expect(result).toBe('');
    });

    it('should handle template IDs that contain -embedded in the middle', () => {
      const result = normalizeTemplateId('my-embedded-app-embedded');

      expect(result).toBe('my-embedded-app');
    });

    it('should handle template IDs with only -embedded as value', () => {
      const result = normalizeTemplateId('-embedded');

      expect(result).toBe('');
    });

    it('should handle template IDs with multiple hyphens', () => {
      const result = normalizeTemplateId('my-custom-template');

      expect(result).toBe('my-custom-template');
    });
  });
});
