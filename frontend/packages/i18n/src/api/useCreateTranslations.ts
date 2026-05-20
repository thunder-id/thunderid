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

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import I18nQueryKeys from '../constants/i18n-query-keys';
import type {CreateTranslationsVariables} from '../models/requests';
import type {TranslationsResponse} from '../models/responses';

/**
 * Custom hook to bulk-create translations for a new language.
 *
 * Sends a single POST request with the full translations bundle to
 * `POST /i18n/languages/{language}/translations`.
 *
 * @returns TanStack Query mutation object for creating translations
 *
 * @example
 * ```tsx
 * function CreateLanguagePage() {
 *   const createTranslations = useCreateTranslations();
 *
 *   const handleCreate = () => {
 *     createTranslations.mutate(
 *       {language: 'fr-FR', translations: {'common': {'hello': 'Bonjour'}}},
 *       {
 *         onSuccess: () => navigate('/translations/fr-FR'),
 *         onError: (error) => console.error('Failed to create:', error),
 *       },
 *     );
 *   };
 * }
 * ```
 */
export default function useCreateTranslations(): UseMutationResult<
  TranslationsResponse,
  Error,
  CreateTranslationsVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<TranslationsResponse, Error, CreateTranslationsVariables>({
    mutationFn: async ({language, translations}: CreateTranslationsVariables): Promise<TranslationsResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {data: TranslationsResponse} = await http.request({
        url: `${serverUrl}/i18n/languages/${language}/translations`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: {translations},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, variables) => {
      void queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS]});
      void queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS, variables.language]});
      void queryClient.invalidateQueries({queryKey: [I18nQueryKeys.LANGUAGES]});
    },
  });
}
