// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {ApplyAllResult} from '../models/promotion';

/**
 * Re-applies every environment's latest version.
 *
 * Editing a value the configuration references, such as a redirect URL, does not change any stored
 * version, so nothing reaches the Data Planes until an apply runs. This is that push.
 */
export default function useApplyAll(): UseMutationResult<{results: ApplyAllResult[]}, Error, void> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<{results: ApplyAllResult[]}, Error, void>({
    mutationFn: async (): Promise<{results: ApplyAllResult[]}> => {
      const response: {data: {results: ApplyAllResult[]}} = await http.request({
        url: `${baseUrl}/apply`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
        data: {},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data: {results: ApplyAllResult[]}) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
        // Ignore invalidation errors.
      });
      const failed: number = (data.results ?? []).filter((r: ApplyAllResult) => Boolean(r.error)).length;
      const applied: number = (data.results ?? []).length - failed;
      if (failed > 0) {
        showToast(
          t('applyAll.partial', 'Applied to {{applied}} environment(s); {{failed}} could not be applied', {
            applied,
            failed,
          }),
          'warning',
        );
        return;
      }
      showToast(t('applyAll.success', 'Applied to {{applied}} environment(s)', {applied}), 'success');
    },
    onError: () => {
      showToast(t('applyAll.error', 'Failed to apply to the environments. Please try again.'), 'error');
    },
  });
}
