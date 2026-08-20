// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import EnvironmentVariableQueryKeys from '../constants/environment-variable-query-keys';
import type {EnvironmentVariable, UpdateEnvironmentVariableVariables} from '../models/environment-variable';

export default function useUpdateEnvironmentVariable(
  envId: string,
): UseMutationResult<EnvironmentVariable, Error, UpdateEnvironmentVariableVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('environmentVariables');
  const {showToast} = useToast();

  return useMutation<EnvironmentVariable, Error, UpdateEnvironmentVariableVariables>({
    mutationFn: async (variables: UpdateEnvironmentVariableVariables): Promise<EnvironmentVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: EnvironmentVariable} = await http.request({
        url: `${serverUrl}/environments/${envId}/variables/${variables.id}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data: variables.data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result: EnvironmentVariable, variables: UpdateEnvironmentVariableVariables) => {
      queryClient
        .invalidateQueries({queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLE, envId, variables.id]})
        .catch(() => {
          // Ignore invalidation errors.
        });
      queryClient
        .invalidateQueries({queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLES, envId]})
        .catch(() => {
          // Ignore invalidation errors.
        });
      showToast(t('update.success', 'Environment variable updated successfully'), 'success');
    },
    onError: () => {
      showToast(t('update.error', 'Failed to update the environment variable. Please try again.'), 'error');
    },
  });
}
