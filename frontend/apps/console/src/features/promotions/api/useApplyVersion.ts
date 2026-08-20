// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {ApplyResult} from '../models/promotion';

/** Variables for an apply. version accepts a number, "latest" or "previous". */
export interface ApplyVariables {
  envId: string;
  version?: string;
}

/**
 * Applies a version to an environment's data plane.
 */
export default function useApplyVersion(): UseMutationResult<ApplyResult, Error, ApplyVariables> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<ApplyResult, Error, ApplyVariables>({
    mutationFn: async (variables: ApplyVariables): Promise<ApplyResult> => {
      const response: {data: ApplyResult} = await http.request({
        url: `${baseUrl}/environments/${variables.envId}/apply`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
        data: {version: variables.version},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (result: ApplyResult) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
        // Ignore invalidation errors.
      });
      // The apply is carried out by the Control Plane pod holding this data plane's connection,
      // which is not always the one that took the request. Reporting success while it is still
      // queued would claim the data plane holds configuration it has not been given yet.
      if (result.status !== 'done') {
        showToast(t('apply.queued', 'Configuration queued and will be applied shortly'), 'info');
        return;
      }
      showToast(t('apply.success', 'Configuration applied successfully'), 'success');
    },
    onError: () => {
      showToast(t('apply.error', 'Failed to apply the configuration. Please try again.'), 'error');
    },
  });
}
