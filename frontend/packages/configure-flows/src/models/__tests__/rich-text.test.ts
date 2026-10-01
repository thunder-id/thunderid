// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect} from 'vitest';
import {UPDATE_TYPES} from '../rich-text';
import type {UpdateType} from '../rich-text';

describe('rich-text', () => {
  describe('UPDATE_TYPES', () => {
    it('should have INTERNAL type with value "internal"', () => {
      expect(UPDATE_TYPES.INTERNAL).toBe('internal');
    });

    it('should have EXTERNAL type with value "external"', () => {
      expect(UPDATE_TYPES.EXTERNAL).toBe('external');
    });

    it('should have NONE type with value "none"', () => {
      expect(UPDATE_TYPES.NONE).toBe('none');
    });

    it('should have exactly three update types', () => {
      const keys = Object.keys(UPDATE_TYPES);
      expect(keys).toHaveLength(3);
      expect(keys).toContain('INTERNAL');
      expect(keys).toContain('EXTERNAL');
      expect(keys).toContain('NONE');
    });

    it('should be immutable (const assertion)', () => {
      const updateType: UpdateType = UPDATE_TYPES.INTERNAL;
      expect(updateType).toBe('internal');

      const externalType: UpdateType = UPDATE_TYPES.EXTERNAL;
      expect(externalType).toBe('external');

      const noneType: UpdateType = UPDATE_TYPES.NONE;
      expect(noneType).toBe('none');
    });
  });
});
