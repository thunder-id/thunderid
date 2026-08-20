// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';

/**
 * Removes an environment and its stored configuration versions from the environment manager. The
 * data planes themselves are left untouched.
 */
export default function useDeleteEnvironment(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (envId: string): Promise<void> => {
      await http.request({
        url: `${baseUrl}/environments/${envId}`,
        method: 'DELETE',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result: void, envId: string) => {
      queryClient.removeQueries({queryKey: [PromotionQueryKeys.VERSIONS, envId]});
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('environment.deleteSuccess', 'Environment removed'), 'success');
    },
    onError: () => {
      showToast(t('environment.deleteError', 'Failed to remove the environment. Please try again.'), 'error');
    },
  });
}
