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
  I18nStorageStrategy,
  createPackageComponentLogger,
  I18nBundle,
  I18nTranslations,
  TranslationBundleConstants,
  getDefaultI18nBundles,
  normalizeTranslations,
} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, useCallback, useEffect, useMemo, useState} from 'react';
import I18nContext, {I18nContextValue} from './I18nContext';

const logger: ReturnType<typeof createPackageComponentLogger> = createPackageComponentLogger(
  '@thunderid/react',
  'I18nProvider',
);

const DEFAULT_STORAGE_KEY = 'thunderid-i18n-language';
const DEFAULT_URL_PARAM = 'lang';

export interface I18nProviderProps {
  /**
   * The i18n preferences from the ThunderIDProvider
   */
  preferences?: I18nPreferences;
}

const detectBrowserLanguage = (): string => {
  if (typeof window !== 'undefined' && window.navigator) {
    return window.navigator.language || TranslationBundleConstants.FALLBACK_LOCALE;
  }

  return TranslationBundleConstants.FALLBACK_LOCALE;
};

const deriveRootDomain = (hostname: string): string => {
  const parts: string[] = hostname.split('.');
  return parts.length > 1 ? parts.slice(-2).join('.') : hostname;
};

const getCookie = (name: string): string | null => {
  if (typeof document === 'undefined') return null;
  const match: RegExpMatchArray | null = new RegExp(
    `(?:^|; )${name.replace(/([.*+?^${}()|[\]\\])/g, '\\$1')}=([^;]*)`,
  ).exec(document.cookie);
  return match ? decodeURIComponent(match[1]) : null;
};

const setCookie = (name: string, value: string, domain: string): void => {
  if (typeof document === 'undefined') return;
  const maxAge: number = 365 * 24 * 60 * 60;
  const secure: string = typeof window !== 'undefined' && window.location.protocol === 'https:' ? '; Secure' : '';
  document.cookie =
    `${encodeURIComponent(name)}=${encodeURIComponent(value)}` +
    `; Max-Age=${maxAge}` +
    `; Path=/` +
    `; Domain=${domain}` +
    `; SameSite=Lax${secure}`;
};

interface StorageAdapter {
  read: () => string | null;
  write: (language: string) => void;
}

const createStorageAdapter = (strategy: I18nStorageStrategy, key: string, cookieDomain?: string): StorageAdapter => {
  switch (strategy) {
    case 'cookie':
      return {
        read: (): string | null => getCookie(key),
        write: (language: string): void => {
          const domain: string =
            cookieDomain ?? (typeof window !== 'undefined' ? deriveRootDomain(window.location.hostname) : '');
          if (domain) setCookie(key, language, domain);
        },
      };
    case 'localStorage':
      return {
        read: (): string | null => {
          if (typeof window === 'undefined' || !window.localStorage) return null;
          try {
            return window.localStorage.getItem(key);
          } catch {
            return null;
          }
        },
        write: (language: string): void => {
          if (typeof window === 'undefined' || !window.localStorage) return;
          try {
            window.localStorage.setItem(key, language);
          } catch {
            logger.warn('Failed to persist language preference to localStorage.');
          }
        },
      };
    case 'none':
    default:
      return {read: (): null => null, write: (): void => {}};
  }
};

const detectUrlParamLanguage = (paramName: string): string | null => {
  if (typeof window === 'undefined') return null;
  try {
    return new URLSearchParams(window.location.search).get(paramName);
  } catch {
    return null;
  }
};

/**
 * I18nProvider component that manages internationalization state and provides
 * translation functions to child components.
 */
const I18nProvider: FC<PropsWithChildren<I18nProviderProps>> = ({
  children,
  preferences,
}: PropsWithChildren<I18nProviderProps>): ReactElement => {
  // Get default bundles from the browser package
  const defaultBundles: Record<string, I18nBundle> = getDefaultI18nBundles();

  const storageStrategy: I18nStorageStrategy = preferences?.storageStrategy ?? 'cookie';
  const storageKey: string = preferences?.storageKey ?? DEFAULT_STORAGE_KEY;
  const urlParamConfig: string | false = preferences?.urlParam === undefined ? DEFAULT_URL_PARAM : preferences.urlParam;

  const resolvedCookieDomain: string | undefined = useMemo((): string | undefined => {
    if (storageStrategy !== 'cookie') return undefined;
    if (preferences?.cookieDomain) return preferences.cookieDomain;
    return typeof window !== 'undefined' ? deriveRootDomain(window.location.hostname) : undefined;
  }, [storageStrategy, preferences?.cookieDomain]);

  const storage: StorageAdapter = useMemo(
    () => createStorageAdapter(storageStrategy, storageKey, resolvedCookieDomain),
    [storageStrategy, storageKey, resolvedCookieDomain],
  );

  const determineInitialLanguage = (): string => {
    if (preferences?.language) return preferences.language;
    if (urlParamConfig !== false) {
      const urlLanguage: string | null = detectUrlParamLanguage(urlParamConfig);
      if (urlLanguage) {
        storage.write(urlLanguage);
        return urlLanguage;
      }
    }
    const storedLanguage: string | null = storage.read();
    if (storedLanguage) return storedLanguage;
    const browserLanguage: string = detectBrowserLanguage();
    if (browserLanguage) return browserLanguage;
    return preferences?.fallbackLanguage || TranslationBundleConstants.FALLBACK_LOCALE;
  };

  const [currentLanguage, setCurrentLanguage] = useState<string>(determineInitialLanguage);

  // Bundles injected at runtime (e.g., from flow metadata i18n translations).
  // These take precedence over defaults but are overridden by prop-provided bundles.
  const [injectedBundles, setInjectedBundles] = useState<Record<string, I18nBundle>>({});

  const injectBundles: (newBundles: Record<string, I18nBundle>) => void = useCallback(
    (newBundles: Record<string, I18nBundle>): void => {
      setInjectedBundles((prev: Record<string, I18nBundle>) => {
        const merged: Record<string, I18nBundle> = {...prev};
        Object.entries(newBundles).forEach(([key, bundle]: [string, I18nBundle]) => {
          const normalizedTranslations: I18nTranslations = normalizeTranslations(
            bundle.translations as unknown as Record<string, string | Record<string, string>>,
          );
          if (merged[key]) {
            merged[key] = {
              ...merged[key],
              translations: deepMerge(merged[key].translations, normalizedTranslations),
            };
          } else {
            merged[key] = {...bundle, translations: normalizedTranslations};
          }
        });
        return merged;
      });
    },
    [],
  );

  /**
   * Merge bundles in priority order: defaults → injected (meta) → prop-provided
   */
  const mergedBundles: Record<string, I18nBundle> = useMemo(() => {
    const merged: Record<string, I18nBundle> = {};

    // 1. Default bundles
    Object.entries(defaultBundles).forEach(([key, bundle]: [string, I18nBundle]) => {
      // Convert key format (e.g., 'en_US' to 'en-US')
      const languageKey: string = key.replace('_', '-');
      merged[languageKey] = bundle;
    });

    // 2. Injected bundles (e.g., from flow metadata) — override defaults
    Object.entries(injectedBundles).forEach(([key, bundle]: [string, I18nBundle]) => {
      const normalizedTranslations: I18nTranslations = normalizeTranslations(
        bundle.translations as unknown as Record<string, string | Record<string, string>>,
      );
      if (merged[key]) {
        merged[key] = {
          ...merged[key],
          translations: deepMerge(merged[key].translations, normalizedTranslations),
        };
      } else {
        merged[key] = {...bundle, translations: normalizedTranslations};
      }
    });

    // 3. User-provided bundles (from props) — highest priority, override everything
    if (preferences?.bundles) {
      Object.entries(preferences.bundles).forEach(([key, userBundle]: [string, I18nBundle]) => {
        const normalizedTranslations: I18nTranslations = normalizeTranslations(
          userBundle.translations as unknown as Record<string, string | Record<string, string>>,
        );
        if (merged[key]) {
          merged[key] = {
            ...merged[key],
            metadata: userBundle.metadata ? {...merged[key].metadata, ...userBundle.metadata} : merged[key].metadata,
            translations: deepMerge(merged[key].translations, normalizedTranslations),
          };
        } else {
          merged[key] = {...userBundle, translations: normalizedTranslations};
        }
      });
    }

    return merged;
  }, [defaultBundles, injectedBundles, preferences?.bundles]);

  const fallbackLanguage: string = preferences?.fallbackLanguage || TranslationBundleConstants.FALLBACK_LOCALE;

  // Persist language changes to the configured storage.
  useEffect(() => {
    storage.write(currentLanguage);
  }, [currentLanguage, storage]);

  // Translation function
  const t: (key: string, params?: Record<string, string | number>) => string = useCallback(
    (key: string, params?: Record<string, string | number>): string => {
      let translation: string | undefined;

      // Try to get translation from current language bundle
      const currentBundle: I18nBundle | undefined = mergedBundles[currentLanguage];
      if (currentBundle?.translations[key]) {
        translation = currentBundle.translations[key];
      }

      // Fallback to fallback language if translation not found
      if (!translation && currentLanguage !== fallbackLanguage) {
        const fallbackBundle: I18nBundle | undefined = mergedBundles[fallbackLanguage];
        if (fallbackBundle?.translations[key]) {
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
    },
    [mergedBundles, currentLanguage, fallbackLanguage],
  );

  // Language setter function
  const setLanguage: (language: string) => void = useCallback(
    (language: string) => {
      if (mergedBundles[language]) {
        setCurrentLanguage(language);
      } else {
        logger.warn(
          `Language '${language}' is not available. Available languages: ${Object.keys(mergedBundles).join(', ')}`,
        );
      }
    },
    [mergedBundles],
  );

  const contextValue: I18nContextValue = useMemo(
    () => ({
      bundles: mergedBundles,
      currentLanguage,
      fallbackLanguage,
      injectBundles,
      setLanguage,
      t,
    }),
    [currentLanguage, fallbackLanguage, injectBundles, mergedBundles, setLanguage, t],
  );

  return <I18nContext.Provider value={contextValue}>{children}</I18nContext.Provider>;
};

export default I18nProvider;
