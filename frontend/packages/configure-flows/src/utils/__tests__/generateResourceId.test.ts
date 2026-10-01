// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, expect, it, vi, beforeEach, afterEach} from 'vitest';
import generateResourceId from '../generateResourceId';

describe('generateResourceId', () => {
  beforeEach(() => {
    vi.spyOn(Math, 'random');
  });

  afterEach(() => {
    vi.restoreAllMocks();
  });

  describe('Basic Functionality', () => {
    it('should return a string with default prefix and character count', () => {
      const id = generateResourceId();

      expect(typeof id).toBe('string');
      expect(id.startsWith('resource_')).toBe(true);
    });

    it('should use the provided prefix', () => {
      const id = generateResourceId('step');

      expect(id.startsWith('step_')).toBe(true);
    });

    it('should use the provided character count', () => {
      vi.mocked(Math.random).mockReturnValue(0.123456789);

      const id = generateResourceId('test', 6);
      const suffix = id.split('_')[1];

      expect(suffix).toHaveLength(6);
    });

    it('should generate the correct format: prefix_randomChars', () => {
      const id = generateResourceId('element', 4);
      const parts = id.split('_');

      expect(parts).toHaveLength(2);
      expect(parts[0]).toBe('element');
      expect(parts[1]).toHaveLength(4);
    });
  });

  describe('Random Generation', () => {
    it('should use Math.random for generating the suffix', () => {
      generateResourceId();

      expect(Math.random).toHaveBeenCalled();
    });

    it('should generate different IDs on multiple calls', () => {
      const id1 = generateResourceId();
      const id2 = generateResourceId();

      expect(id1).not.toBe(id2);
    });

    it('should generate alphanumeric characters in the suffix', () => {
      const ids = Array.from({length: 10}, () => generateResourceId());

      ids.forEach((id) => {
        const suffix = id.split('_')[1];
        expect(suffix).toMatch(/^[a-z0-9]+$/);
      });
    });

    it('should generate consistent output for mocked random value', () => {
      vi.mocked(Math.random).mockReturnValue(0.5);

      const id1 = generateResourceId('test', 4);

      vi.mocked(Math.random).mockReturnValue(0.5);

      const id2 = generateResourceId('test', 4);

      expect(id1).toBe(id2);
    });
  });

  describe('Default Parameters', () => {
    it('should use "resource" as default prefix', () => {
      const id = generateResourceId();

      expect(id.startsWith('resource_')).toBe(true);
    });

    it('should use 4 as default character count', () => {
      vi.mocked(Math.random).mockReturnValue(0.123456789);

      const id = generateResourceId();
      const suffix = id.split('_')[1];

      expect(suffix).toHaveLength(4);
    });

    it('should allow overriding only the prefix', () => {
      const id = generateResourceId('custom');

      expect(id.startsWith('custom_')).toBe(true);
      expect(id.split('_')[1]).toHaveLength(4);
    });
  });

  describe('Edge Cases', () => {
    it('should handle empty string prefix', () => {
      const id = generateResourceId('');

      expect(id.startsWith('_')).toBe(true);
    });

    it('should handle zero character count', () => {
      const id = generateResourceId('test', 0);

      expect(id).toBe('test_');
    });

    it('should handle character count of 1', () => {
      vi.mocked(Math.random).mockReturnValue(0.123456789);

      const id = generateResourceId('test', 1);
      const suffix = id.split('_')[1];

      expect(suffix).toHaveLength(1);
    });

    it('should handle large character counts', () => {
      vi.mocked(Math.random).mockReturnValue(0.123456789);

      const id = generateResourceId('test', 10);
      const suffix = id.split('_')[1];

      // Math.random().toString(36) produces limited characters, so it caps at available length
      expect(suffix.length).toBeLessThanOrEqual(10);
    });

    it('should handle prefix with special characters', () => {
      const id = generateResourceId('my-prefix');

      expect(id.startsWith('my-prefix_')).toBe(true);
    });
  });
});
