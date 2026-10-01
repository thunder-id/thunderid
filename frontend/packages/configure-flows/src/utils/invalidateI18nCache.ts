// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

const I18N_CACHE_INVALIDATE_KEY = '__I18nCacheInvalidate__';

interface WindowWithI18nCache extends Window {
  [I18N_CACHE_INVALIDATE_KEY]?: () => void;
}

export function invalidateI18nCache(): void {
  const fn = (window as WindowWithI18nCache)[I18N_CACHE_INVALIDATE_KEY];
  fn?.();
}
