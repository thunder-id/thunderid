/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {
  deepMerge,
  I18nPreferences,
  Preferences,
  I18nBundle,
  I18nTranslations,
  normalizeTranslations,
} from '@thunderid/browser';
import {useContext, useMemo} from 'react';
import ComponentPreferencesContext from '../contexts/I18n/ComponentPreferencesContext';
import I18nContext from '../contexts/I18n/I18nContext';

export interface UseTranslation {
  /**
   * All available language codes
   */
  availableLanguages: string[];

  /**
   * The current language code
   */
  currentLanguage: string;

  /**
   * Function to change the current language
   */
  setLanguage: (language: string) => void;

  /**
   * Translation function that returns a translated string for the given key
   */
  t: (key: string, params?: Record<string, string | number>) => string;
}

export interface UseTranslationWithPreferences extends UseTranslation {
  /**
   * Enhanced translation function that merges component-level preferences
   */
  t: (key: string, params?: Record<string, string | number>) => string;
}

/**
 * Hook for accessing translation functions and language management.
 * Must be used within an I18nProvider context.
 *
 * @param componentPreferences - Optional component-level i18n preferences to merge with global ones
 * @returns An object containing translation function and language management utilities
 * @throws Error if used outside of I18nProvider context
 */
const useTranslation = (componentPreferences?: I18nPreferences): UseTranslationWithPreferences => {
  const context: any = useContext(I18nContext);
  const componentPrefs: Preferences | undefined = useContext(ComponentPreferencesContext);
  const contextPreferences: I18nPreferences | undefined = componentPrefs?.i18n;

  if (!context) {
    throw new Error(
      'useTranslation must be used within an I18nProvider. Make sure your component is wrapped with ThunderIDProvider which includes I18nProvider.',
    );
  }

  // Direct parameter takes precedence over context-provided preferences
  const effectivePreferences: I18nPreferences | undefined = componentPreferences ?? contextPreferences;

  const {t: globalT, currentLanguage, setLanguage, bundles: globalBundles, fallbackLanguage} = context;

  // Merge global bundles with component-level bundles if provided
  const mergedBundles: Record<string, I18nBundle> = useMemo(() => {
    if (!effectivePreferences?.bundles) {
      return globalBundles;
    }

    const merged: Record<string, I18nBundle> = {};

    // Start with global bundles
    Object.entries(globalBundles).forEach(([key, bundle]: [string, I18nBundle]) => {
      merged[key] = bundle;
    });

    // Merge component-level bundles using deepMerge for better merging
    Object.entries(effectivePreferences.bundles).forEach(([key, componentBundle]: [string, I18nBundle]) => {
      const normalizedTranslations: I18nTranslations = normalizeTranslations(
        componentBundle.translations as unknown as Record<string, string | Record<string, string>>,
      );
      if (merged[key]) {
        // Deep merge component bundle with existing global bundle
        merged[key] = {
          ...merged[key],
          metadata: componentBundle.metadata
            ? {...merged[key].metadata, ...componentBundle.metadata}
            : merged[key].metadata,
          translations: deepMerge(merged[key].translations, normalizedTranslations),
        };
      } else {
        // No global bundle for this language, use component bundle as-is
        merged[key] = {...componentBundle, translations: normalizedTranslations};
      }
    });

    return merged;
  }, [globalBundles, effectivePreferences?.bundles]);

  // Create enhanced translation function that uses merged bundles
  const enhancedT: (key: string, params?: Record<string, string | number>) => string = useMemo(() => {
    if (!effectivePreferences?.bundles) {
      // No component preferences, use global translation function
      return globalT;
    }

    return (key: string, params?: Record<string, string | number>): string => {
      let translation: string | undefined;

      // Try to get translation from current language bundle
      const currentBundle: I18nBundle | undefined = mergedBundles[currentLanguage];
      if (currentBundle?.translations?.[key]) {
        translation = currentBundle.translations[key];
      }

      // Fallback to fallback language if translation not found
      if (!translation && currentLanguage !== fallbackLanguage) {
        const fallbackBundle: I18nBundle | undefined = mergedBundles[fallbackLanguage];
        if (fallbackBundle?.translations?.[key]) {
          translation = fallbackBundle.translations[key];
        }
      }

      // If still no translation found, return the key itself
      if (!translation) {
        translation = key;
      }

      // Replace parameters if provided
      if (params && Object.keys(params).length > 0) {
        return Object.entries(params).reduce(
          (acc: string, [paramKey, paramValue]: [string, string | number]) =>
            acc.replace(new RegExp(`\\{${paramKey}\\}`, 'g'), String(paramValue)),
          translation,
        );
      }

      return translation;
    };
  }, [mergedBundles, currentLanguage, fallbackLanguage, globalT, effectivePreferences?.bundles]);

  return {
    availableLanguages: Object.keys(mergedBundles),
    currentLanguage,
    setLanguage,
    t: enhancedT,
  };
};

export default useTranslation;
