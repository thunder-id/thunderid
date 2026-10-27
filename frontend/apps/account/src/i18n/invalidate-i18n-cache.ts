// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

/**
 * Global key for the i18n cache invalidation function.
 */
const I18N_CACHE_INVALIDATE_KEY = '__I18nCacheInvalidate__';

/**
 * Type for the window with i18n cache invalidation function.
 */
interface WindowWithI18nCache extends Window {
  [I18N_CACHE_INVALIDATE_KEY]?: () => void;
}

/**
 * Registers the i18n cache invalidation function.
 * Called by I18nProvider to expose the invalidation function.
 *
 * @param invalidateFn - Function to invalidate the i18n cache
 */
export function registerI18nCacheInvalidator(invalidateFn: () => void): void {
  (window as WindowWithI18nCache)[I18N_CACHE_INVALIDATE_KEY] = invalidateFn;
}

/**
 * Unregisters the i18n cache invalidation function.
 * Called by I18nProvider on cleanup.
 */
export function unregisterI18nCacheInvalidator(): void {
  delete (window as WindowWithI18nCache)[I18N_CACHE_INVALIDATE_KEY];
}

/**
 * Invalidates the i18n translations cache, triggering a refetch from the API.
 * Call this after creating or updating translations.
 */
export function invalidateI18nCache(): void {
  (window as WindowWithI18nCache)[I18N_CACHE_INVALIDATE_KEY]?.();
}
