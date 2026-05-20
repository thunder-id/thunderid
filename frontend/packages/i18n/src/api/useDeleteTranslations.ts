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

/**
 * Custom hook to delete all custom translations for a language.
 *
 * Calls DELETE /i18n/languages/{language}/translations which removes all
 * custom translation overrides for the language, resetting it to defaults.
 *
 * @returns TanStack Query mutation object for deleting translations
 *
 * @example
 * ```tsx
 * function DeleteLanguageButton({ language }: { language: string }) {
 *   const deleteTranslations = useDeleteTranslations();
 *
 *   const handleDelete = () => {
 *     deleteTranslations.mutate(language, {
 *       onSuccess: () => console.log('Translations deleted'),
 *       onError: (error) => console.error('Failed to delete:', error),
 *     });
 *   };
 *
 *   return (
 *     <button onClick={handleDelete} disabled={deleteTranslations.isPending}>
 *       {deleteTranslations.isPending ? 'Deleting...' : 'Delete Language'}
 *     </button>
 *   );
 * }
 * ```
 */
export default function useDeleteTranslations(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (language: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/i18n/languages/${language}/translations`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, language) => {
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS, language]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.LANGUAGES]}).catch(() => {
        // Ignore invalidation errors
      });
    },
  });
}
