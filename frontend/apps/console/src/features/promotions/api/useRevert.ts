// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {RevertResult} from '../models/promotion';

/** Variables for a revert. toVersion accepts a version number or "previous". */
export interface RevertVariables {
  envId: string;
  toVersion: string;
  apply?: boolean;
  note?: string;
}

/**
 * Reverts an environment to an earlier version. Reverting adds a new version restoring the older
 * content rather than deleting history.
 */
export default function useRevert(): UseMutationResult<RevertResult, Error, RevertVariables> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<RevertResult, Error, RevertVariables>({
    mutationFn: async (variables: RevertVariables): Promise<RevertResult> => {
      const response: {data: RevertResult} = await http.request({
        url: `${baseUrl}/environments/${variables.envId}/revert`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
        data: {toVersion: variables.toVersion, apply: variables.apply, note: variables.note},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result: RevertResult, variables: RevertVariables) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.ENVIRONMENTS]}).catch(() => {
        // Ignore invalidation errors.
      });
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.VERSIONS, variables.envId]}).catch(() => {
        // Ignore invalidation errors.
      });
      // A revert records a new version restoring an earlier one. It reaches the running deployment
      // only when the environment is applied.
      showToast(t('revert.success', 'Environment reverted successfully'), 'success');
    },
    onError: () => {
      showToast(t('revert.error', 'Failed to revert the environment. Please try again.'), 'error');
    },
  });
}
