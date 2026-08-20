// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import useEnvManagerUrl from './useEnvManagerUrl';
import PromotionQueryKeys from '../constants/promotion-query-keys';
import type {SecretEntry} from '../models/promotion';

/** Variables for setting one credential. */
export interface SetSecretVariables {
  envId: string;
  name: string;
  value: string;
}

/**
 * Writes a credential to an environment's Data Plane.
 *
 * The request carries only the value: whether it is stored as a hash or as is follows from the
 * configuration that uses it, so it cannot be chosen here and cannot drift from what the Data Plane
 * expects to find.
 */
export default function useSetEnvironmentSecret(): UseMutationResult<SecretEntry, Error, SetSecretVariables> {
  const {http} = useThunderID();
  const baseUrl: string | undefined = useEnvManagerUrl();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('promotions');
  const {showToast} = useToast();

  return useMutation<SecretEntry, Error, SetSecretVariables>({
    mutationFn: async (variables: SetSecretVariables): Promise<SecretEntry> => {
      const response: {data: SecretEntry} = await http.request({
        url: `${baseUrl}/environments/${variables.envId}/secrets/${encodeURIComponent(variables.name)}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        credentials: 'same-origin',
        data: {value: variables.value},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data: SecretEntry, variables: SetSecretVariables) => {
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.SECRETS, variables.envId]}).catch(() => {
        // Ignore invalidation errors.
      });
      queryClient.invalidateQueries({queryKey: [PromotionQueryKeys.VARIABLES, variables.envId]}).catch(() => {
        // Ignore invalidation errors.
      });
      showToast(t('secrets.setSuccess', 'Secret stored on the Data Plane'), 'success');
    },
    onError: (error: Error) => {
      showToast(error.message || t('secrets.setError', 'Failed to store the secret'), 'error');
    },
  });
}
