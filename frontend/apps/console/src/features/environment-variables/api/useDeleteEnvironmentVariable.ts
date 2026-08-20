// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import EnvironmentVariableQueryKeys from '../constants/environment-variable-query-keys';

export default function useDeleteEnvironmentVariable(envId: string): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('environmentVariables');
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/environments/${envId}/variables/${id}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result: void, id: string) => {
      queryClient.removeQueries({queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLE, envId, id]});
      queryClient
        .invalidateQueries({queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLES, envId]})
        .catch(() => {
          // Ignore invalidation errors.
        });
      showToast(t('delete.success', 'Environment variable deleted successfully'), 'success');
    },
    onError: () => {
      showToast(t('delete.error', 'Failed to delete the environment variable. Please try again.'), 'error');
    },
  });
}
