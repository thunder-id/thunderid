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

import {useQuery, useQueryClient} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {I18nQueryKeys} from '@thunderid/i18n';
import {useThunderID} from '@thunderid/react';
import {type ReactElement, type ReactNode, useEffect} from 'react';
import {useTranslation} from 'react-i18next';
import {registerI18nCacheInvalidator, unregisterI18nCacheInvalidator} from './invalidate-i18n-cache';

/**
 * Response from the translations API.
 * The translations object is keyed by namespace, then by translation key.
 */
interface TranslationsResponse {
  language: string;
  totalResults?: number;
  translations: Record<string, Record<string, string>>;
}

/**
 * Props for I18nProvider.
 */
export interface I18nProviderProps {
  /**
   * The children to render.
   */
  children: ReactNode;
}

/**
 * Provider component that fetches all translations from the i18n API and adds them to i18next.
 * API translations take precedence over static translations.
 *
 * Translations are fetched without namespace filtering, allowing all namespaces to be merged
 * into i18next at their respective namespace levels.
 *
 * This provider should be placed inside the AuthProvider and ConfigProvider.
 *
 * @example
 * ```tsx
 * <AuthProvider>
 *   <ConfigProvider>
 *     <QueryClientProvider>
 *       <I18nProvider>
 *         <App />
 *       </I18nProvider>
 *     </QueryClientProvider>
 *   </ConfigProvider>
 * </AuthProvider>
 * ```
 */
function I18nProvider({children}: I18nProviderProps): ReactElement {
  const {i18n} = useTranslation();
  const {http} = useThunderID() as unknown as {
    http: {
      request: (config: {
        url: string;
        method: string;
        attachToken?: boolean;
        credentials?: RequestCredentials;
      }) => Promise<{data: TranslationsResponse}>;
    };
  };
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  // Get current language (e.g., 'en-US')
  const currentLanguage = i18n.language;

  // Fetch all translations from API (no namespace filter)
  const {data: apiTranslations} = useQuery<TranslationsResponse, Error>({
    queryKey: [I18nQueryKeys.TRANSLATIONS, currentLanguage],
    queryFn: async (): Promise<TranslationsResponse> => {
      const serverUrl = getServerUrl();
      const url = `${serverUrl}/i18n/languages/${currentLanguage}/translations/resolve`;

      const response = await http.request({
        url,
        method: 'GET',
        attachToken: false,
        credentials: 'omit',
      });

      return response.data;
    },
    staleTime: 5 * 60 * 1000, // 5 minutes
  });

  // Add API translations to i18next when they're fetched
  useEffect(() => {
    if (!apiTranslations?.translations) {
      return;
    }

    const i18nLanguage = i18n.language;
    let translationsAdded = false;

    // Iterate through each namespace from the API response
    Object.entries(apiTranslations.translations).forEach(([apiNamespace, namespaceTranslations]) => {
      if (!namespaceTranslations || Object.keys(namespaceTranslations).length === 0) {
        return;
      }

      // Get existing translations for this namespace (from static bundle)
      const existingBundle =
        (i18n.getResourceBundle(i18nLanguage, apiNamespace) as Record<string, string> | undefined) ?? {};

      // Shallow merge: combine static bundle with API translations (API takes precedence for same keys)
      const mergedBundle = {
        ...existingBundle,
        ...namespaceTranslations,
      };

      // Add the merged bundle to the namespace
      i18n.addResourceBundle(i18nLanguage, apiNamespace, mergedBundle, true, true);
      translationsAdded = true;
    });

    // Emit 'added' event to trigger re-render of components listening for translation changes
    if (translationsAdded) {
      i18n.emit('added', i18nLanguage, Object.keys(apiTranslations.translations));
    }
  }, [apiTranslations, i18n]);

  // Expose a method to invalidate translations cache (useful after creating new translations)
  useEffect(() => {
    // Register the invalidate function globally so other components can use it
    registerI18nCacheInvalidator(() => {
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
    });

    return () => {
      unregisterI18nCacheInvalidator();
    };
  }, [queryClient]);

  return children as ReactElement;
}

export default I18nProvider;
