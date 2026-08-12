// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
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
