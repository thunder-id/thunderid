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

import {useThunderID} from '@thunderid/react';
import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import I18nQueryKeys from '../constants/i18n-query-keys';
import type {UpdateTranslationVariables} from '../models/requests';
import type {TranslationResponse} from '../models/responses';

/**
 * Options for the useUpdateTranslation hook.
 */
export interface UseUpdateTranslationOptions {
  /**
   * Optional callback to be called after a successful mutation.
   * This is useful for app-specific cache invalidation (e.g., invalidating i18next cache).
   */
  onMutationSuccess?: (data: TranslationResponse, variables: UpdateTranslationVariables) => void;
}

/**
 * Custom hook to create or update a single translation.
 *
 * @param options - Options for the mutation
 * @returns TanStack Query mutation object for updating translations
 *
 * @example
 * ```tsx
 * function CreateTranslationForm() {
 *   const updateTranslation = useUpdateTranslation({
 *     onMutationSuccess: () => {
 *       // Invalidate app-specific caches
 *       invalidateI18nCache();
 *     },
 *   });
 *
 *   const handleSubmit = (data: UpdateTranslationVariables) => {
 *     updateTranslation.mutate(data, {
 *       onSuccess: (translation) => {
 *         console.log('Translation created:', translation);
 *       },
 *       onError: (error) => {
 *         console.error('Failed to create translation:', error);
 *       }
 *     });
 *   };
 *
 *   return <form onSubmit={handleSubmit}>...</form>;
 * }
 * ```
 */
export default function useUpdateTranslation(
  options?: UseUpdateTranslationOptions,
): UseMutationResult<TranslationResponse, Error, UpdateTranslationVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {onMutationSuccess} = options ?? {};

  return useMutation<TranslationResponse, Error, UpdateTranslationVariables>({
    mutationFn: async ({language, namespace, key, value}: UpdateTranslationVariables): Promise<TranslationResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: TranslationResponse;
      } = await http.request({
        url: `${serverUrl}/i18n/languages/${language}/translations/ns/${namespace}/keys/${key}`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data: {value},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data, variables) => {
      // Invalidate translations cache after successful update
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient.invalidateQueries({queryKey: [I18nQueryKeys.TRANSLATIONS, variables.language]}).catch(() => {
        // Ignore invalidation errors
      });

      // Call app-specific success handler if provided
      onMutationSuccess?.(data, variables);
    },
  });
}
