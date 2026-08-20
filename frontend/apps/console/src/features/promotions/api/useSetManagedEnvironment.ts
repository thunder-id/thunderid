// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {Environment} from '../models/promotion';

/**
 * Moves the mark for the environment the Control Plane administers directly.
 *
 * Exactly one environment of an organization holds it, so this moves the mark rather than toggling
 * it: an organization is never left without one, which would strand every credential created
 * afterwards.
 */
export default function useSetManagedEnvironment(): UseMutationResult<Environment, Error, string> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<Environment, Error, string>({
    mutationFn: async (envId: string): Promise<Environment> => {
      const response: {data: Environment} = await http.request({
        url: `${baseUrl}/environments/${envId}/managed`,
        method: 'POST',
        credentials: 'same-origin',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (result: Environment) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('managed.success', '{{name}} is now managed by the Control Plane', {name: result.name}), 'success');
    },
    onError: () => {
      showToast(t('managed.error', 'Failed to change the managed environment. Please try again.'), 'error');
    },
  });
}
