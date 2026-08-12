// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import I18nQueryKeys from '../constants/i18n-query-keys';
import type {TranslationsResponse} from '../models/responses';

/**
 * Options for the useGetTranslations hook.
 */
export interface UseGetTranslationsOptions {
  /**
   * Language code to fetch translations for.
   */
  language: string;
  /**
   * Optional namespace to filter translations.
   */
  namespace?: string;
  /**
   * Whether the query should be enabled. Defaults to true.
   */
  enabled?: boolean;
}

/**
 * Custom hook to fetch translations for a language.
 *
 * @param options - Options for fetching translations
 * @returns TanStack Query object for fetching translations
 *
 * @example
 * ```tsx
 * function TranslationsDisplay() {
 *   const { data, isLoading, error } = useGetTranslations({
 *     language: 'en',
 *     namespace: 'flowCustomI18n',
 *   });
 *
 *   if (isLoading) return <Spinner />;
 *   if (error) return <Error message={error.message} />;
 *
 *   return (
 *     <ul>
 *       {Object.entries(data?.translations || {}).map(([ns, keys]) => (
 *         Object.entries(keys).map(([key, value]) => (
 *           <li key={`${ns}.${key}`}>{key}: {value}</li>
 *         ))
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useGetTranslations({
  language,
  namespace,
  enabled = true,
}: UseGetTranslationsOptions): UseQueryResult<TranslationsResponse, Error> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<TranslationsResponse, Error>({
    queryKey: namespace ? [I18nQueryKeys.TRANSLATIONS, language, namespace] : [I18nQueryKeys.TRANSLATIONS, language],
    queryFn: async (): Promise<TranslationsResponse> => {
      const serverUrl: string = getServerUrl();
      let url = `${serverUrl}/i18n/languages/${language}/translations/resolve`;

      if (namespace) {
        url += `?namespace=${encodeURIComponent(namespace)}`;
      }

      const response: {
        data: TranslationsResponse;
      } = await http.request({
        url,
        method: 'GET',
        attachToken: false,
        credentials: 'omit',
      });

      return response.data;
    },
    enabled: enabled && !!language,
  });
}
