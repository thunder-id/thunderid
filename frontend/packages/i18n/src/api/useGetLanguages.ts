// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import I18nQueryKeys from '../constants/i18n-query-keys';
import type {LanguagesResponse} from '../models/responses';

/**
 * Options for the useGetLanguages hook.
 */
export interface UseGetLanguagesOptions {
  /**
   * Whether the query should be enabled. Defaults to true.
   */
  enabled?: boolean;
}

/**
 * Custom hook to fetch available languages.
 *
 * @param options - Options for fetching languages
 * @returns TanStack Query object for fetching languages
 *
 * @example
 * ```tsx
 * function LanguageSelector() {
 *   const { data, isLoading, error } = useGetLanguages();
 *
 *   if (isLoading) return <Spinner />;
 *   if (error) return <Error message={error.message} />;
 *
 *   return (
 *     <Select>
 *       {data?.languages.map(lang => (
 *         <Option key={lang} value={lang}>{lang}</Option>
 *       ))}
 *     </Select>
 *   );
 * }
 * ```
 */
export default function useGetLanguages(options?: UseGetLanguagesOptions): UseQueryResult<LanguagesResponse, Error> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {enabled = true} = options ?? {};

  return useQuery<LanguagesResponse, Error>({
    queryKey: [I18nQueryKeys.LANGUAGES],
    queryFn: async (): Promise<LanguagesResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: LanguagesResponse;
      } = await http.request({
        url: `${serverUrl}/i18n/languages`,
        method: 'GET',
        attachToken: false,
        credentials: 'omit',
      });

      return response.data;
    },
    enabled,
  });
}
