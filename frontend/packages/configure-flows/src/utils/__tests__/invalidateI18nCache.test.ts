// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {afterEach, describe, expect, it, vi} from 'vitest';
import {invalidateI18nCache} from '../invalidateI18nCache';

const I18N_CACHE_INVALIDATE_KEY = '__I18nCacheInvalidate__';

describe('invalidateI18nCache', () => {
  afterEach(() => {
    delete (window as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY];
  });

  it('should call the registered invalidate function when present', () => {
    const invalidate = vi.fn();
    (window as Record<string, unknown>)[I18N_CACHE_INVALIDATE_KEY] = invalidate;

    invalidateI18nCache();

    expect(invalidate).toHaveBeenCalledOnce();
  });

  it('should not throw when no invalidate function is registered', () => {
    expect(() => invalidateI18nCache()).not.toThrow();
  });
});
