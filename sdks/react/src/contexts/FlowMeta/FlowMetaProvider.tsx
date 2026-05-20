/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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
  FlowMetadataResponse,
  FlowMetaType,
  getFlowMetaV2,
  Platform,
  I18nBundle,
  TranslationBundleConstants,
} from '@thunderid/browser';
import {FC, PropsWithChildren, ReactElement, RefObject, useCallback, useEffect, useRef, useState} from 'react';
import FlowMetaContext from './FlowMetaContext';
import {I18nContextValue} from '../I18n/I18nContext';
import useI18n from '../I18n/useI18n';
import useThunderID from '../ThunderID/useThunderID';

export interface FlowMetaProviderProps {
  /**
   * When false the provider skips fetching and provides null meta.
   * @default true
   */
  enabled?: boolean;
}

/**
 * FlowMetaProvider fetches flow metadata from the `GET /flow/meta` endpoint
 * (v2 API) and makes it available to child components through `FlowMetaContext`.
 *
 * It is designed to be used in v2 embedded-flow scenarios and integrates with
 * `ThemeProvider` so that theme settings (colors, direction, typography, …)
 * from the server-side design configuration are applied automatically.
 *
 * @example
 * ```tsx
 * <FlowMetaProvider
 *   config={{
 *     baseUrl: 'https://localhost:8090',
 *     type: FlowMetaType.App,
 *     id: 'your-app-id',
 *   }}
 * >
 *   <ThemeProvider>
 *     <App />
 *   </ThemeProvider>
 * </FlowMetaProvider>
 * ```
 */
const FlowMetaProvider: FC<PropsWithChildren<FlowMetaProviderProps>> = ({
  children,
  enabled = true,
}: PropsWithChildren<FlowMetaProviderProps>): ReactElement => {
  const {baseUrl, applicationId, platform, isInitialized} = useThunderID();
  const i18nContext: I18nContextValue = useI18n();

  const [meta, setMeta] = useState<FlowMetadataResponse | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [error, setError] = useState<Error | null>(null);
  const [pendingLanguage, setPendingLanguage] = useState<string | null>(null);

  // Track the last fetchFlowMeta reference that was actually dispatched.
  // This prevents two classes of double-fetch:
  //   1. React StrictMode simulates unmount+remount — the re-mount fires the
  //      effect again with the same fetchFlowMeta reference; without this guard
  //      the else-branch would issue a redundant second network request.
  //   2. Rapid dependency changes (e.g. baseUrl stabilising) that produce two
  //      effect firings before the first fetch completes.
  const lastFetchedRef: RefObject<(() => Promise<void>) | null> = useRef<(() => Promise<void>) | null>(null);

  const fetchFlowMeta: () => Promise<void> = useCallback(async (): Promise<void> => {
    if (!enabled || platform !== Platform.ThunderID) {
      setMeta(null);
      setIsLoading(false);
      return;
    }

    // Defer until ThunderID finishes initializing (e.g. loading applicationId
    // from storage on refresh). Once initialized, proceed even if applicationId
    // is absent — some flows (e.g. AcceptInvite) have no applicationId by design.
    if (!isInitialized && !applicationId) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      const result: FlowMetadataResponse = await getFlowMetaV2({
        baseUrl,
        ...(applicationId ? {id: applicationId, type: FlowMetaType.App} : {}),
        language: i18nContext?.currentLanguage,
      });
      setMeta(result);
    } catch (err: unknown) {
      setError(err instanceof Error ? err : new Error(String(err)));
    } finally {
      setIsLoading(false);
    }
  }, [enabled, platform, baseUrl, applicationId, isInitialized, i18nContext?.currentLanguage]);

  const switchLanguage: (language: string) => Promise<void> = useCallback(
    async (language: string): Promise<void> => {
      if (!enabled || platform !== Platform.ThunderID) return;

      setIsLoading(true);
      setError(null);

      try {
        const result: FlowMetadataResponse = await getFlowMetaV2({
          baseUrl,
          ...(applicationId ? {id: applicationId, type: FlowMetaType.App} : {}),
          language,
        });

        // Inject translations for the new language before switching
        if (result.i18n?.translations && i18nContext?.injectBundles) {
          const flatTranslations: Record<string, string> = {};
          Object.entries(result.i18n.translations).forEach(([namespace, keys]: [string, Record<string, string>]) => {
            Object.entries(keys).forEach(([key, value]: [string, string]) => {
              flatTranslations[`${namespace}.${key}`] = value;
            });
          });
          const bundle: I18nBundle = {translations: flatTranslations} as unknown as I18nBundle;
          i18nContext.injectBundles({[language]: bundle});
        }

        // Defer setLanguage to the next effect cycle so injectBundles state
        // is committed before I18nProvider's setLanguage checks mergedBundles.
        setPendingLanguage(language);
        setMeta(result);
      } catch (err: unknown) {
        setError(err instanceof Error ? err : new Error(String(err)));
      } finally {
        setIsLoading(false);
      }
    },
    [enabled, platform, baseUrl, applicationId, i18nContext],
  );

  // After injectBundles + setPendingLanguage are batched and committed, this
  // effect fires with the updated i18nContext (mergedBundles now includes the
  // new language), so setLanguage succeeds on the first switch.
  useEffect(() => {
    if (pendingLanguage && i18nContext?.setLanguage) {
      i18nContext.setLanguage(pendingLanguage);
      setPendingLanguage(null);
    }
  }, [pendingLanguage, i18nContext?.setLanguage]);

  useEffect(() => {
    if (lastFetchedRef.current === fetchFlowMeta) {
      // Same reference as the last dispatch — this is a StrictMode re-mount
      // or an effect re-fire with unchanged deps. Skip to avoid a duplicate fetch.
      return;
    }

    lastFetchedRef.current = fetchFlowMeta;
    fetchFlowMeta();
  }, [fetchFlowMeta]);

  // When meta loads with i18n translations, inject them into the i18n system.
  // Meta translations act as the base layer — prop-provided bundles still take precedence.
  useEffect(() => {
    if (!meta?.i18n?.translations || !i18nContext?.injectBundles) {
      return;
    }

    const metaLanguage: string = meta.i18n.language || TranslationBundleConstants.FALLBACK_LOCALE;

    // Flatten namespace-keyed translations to dot-path keys:
    // { "signin": { "heading": "Sign In" } } → { "signin.heading": "Sign In" }
    const flatTranslations: Record<string, string> = {};
    Object.entries(meta.i18n.translations).forEach(([namespace, keys]: [string, Record<string, string>]) => {
      Object.entries(keys).forEach(([key, value]: [string, string]) => {
        flatTranslations[`${namespace}.${key}`] = value;
      });
    });

    const bundle: I18nBundle = {translations: flatTranslations} as unknown as I18nBundle;

    // Inject under the meta language code and the i18n current language so
    // lookups succeed regardless of whether the system uses "en" or "en-US".
    const bundlesToInject: Record<string, I18nBundle> = {[metaLanguage]: bundle};
    if (i18nContext.currentLanguage && i18nContext.currentLanguage !== metaLanguage) {
      bundlesToInject[i18nContext.currentLanguage] = bundle;
    }
    if (i18nContext.fallbackLanguage && i18nContext.fallbackLanguage !== metaLanguage) {
      bundlesToInject[i18nContext.fallbackLanguage] = bundle;
    }

    i18nContext.injectBundles(bundlesToInject);
  }, [meta?.i18n?.translations, i18nContext?.injectBundles]);

  const value: any = {
    error,
    fetchFlowMeta,
    isLoading,
    meta,
    switchLanguage,
  };

  return <FlowMetaContext.Provider value={value}>{children}</FlowMetaContext.Provider>;
};

export default FlowMetaProvider;
