// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {describe, it, expect, vi, beforeEach} from 'vitest';
import {
  registerI18nCacheInvalidator,
  unregisterI18nCacheInvalidator,
  invalidateI18nCache,
} from '../invalidate-i18n-cache';

describe('invalidate-i18n-cache', () => {
  const I18N_CACHE_INVALIDATE_KEY = '__I18nCacheInvalidate__';

  beforeEach(() => {
    // Clean up the global key before each test
    delete (window as unknown as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY];
  });

  describe('registerI18nCacheInvalidator', () => {
    it('should register the invalidation function on window', () => {
      const mockInvalidateFn = vi.fn();

      registerI18nCacheInvalidator(mockInvalidateFn);

      expect((window as unknown as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY]).toBe(mockInvalidateFn);
    });

    it('should overwrite existing invalidation function', () => {
      const firstFn = vi.fn();
      const secondFn = vi.fn();

      registerI18nCacheInvalidator(firstFn);
      registerI18nCacheInvalidator(secondFn);

      expect((window as unknown as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY]).toBe(secondFn);
    });
  });

  describe('unregisterI18nCacheInvalidator', () => {
    it('should remove the invalidation function from window', () => {
      const mockInvalidateFn = vi.fn();
      registerI18nCacheInvalidator(mockInvalidateFn);

      unregisterI18nCacheInvalidator();

      expect((window as unknown as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY]).toBeUndefined();
    });

    it('should not throw when no function is registered', () => {
      expect(() => unregisterI18nCacheInvalidator()).not.toThrow();
    });
  });

  describe('invalidateI18nCache', () => {
    it('should call the registered invalidation function', () => {
      const mockInvalidateFn = vi.fn();
      registerI18nCacheInvalidator(mockInvalidateFn);

      invalidateI18nCache();

      expect(mockInvalidateFn).toHaveBeenCalledTimes(1);
    });

    it('should not throw when no function is registered', () => {
      expect(() => invalidateI18nCache()).not.toThrow();
    });

    it('should not call function after it has been unregistered', () => {
      const mockInvalidateFn = vi.fn();
      registerI18nCacheInvalidator(mockInvalidateFn);
      unregisterI18nCacheInvalidator();

      invalidateI18nCache();

      expect(mockInvalidateFn).not.toHaveBeenCalled();
    });
  });
});
