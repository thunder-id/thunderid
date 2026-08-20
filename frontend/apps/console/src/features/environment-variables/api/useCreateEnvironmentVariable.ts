// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import EnvironmentVariableQueryKeys from '../constants/environment-variable-query-keys';
import type {CreateEnvironmentVariableRequest, EnvironmentVariable} from '../models/environment-variable';

export default function useCreateEnvironmentVariable(
  envId: string,
): UseMutationResult<EnvironmentVariable, Error, CreateEnvironmentVariableRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('environmentVariables');
  const {showToast} = useToast();

  return useMutation<EnvironmentVariable, Error, CreateEnvironmentVariableRequest>({
    mutationFn: async (data: CreateEnvironmentVariableRequest): Promise<EnvironmentVariable> => {
      const serverUrl: string = getServerUrl();
      const response: {data: EnvironmentVariable} = await http.request({
        url: `${serverUrl}/environments/${envId}/variables`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient
        .invalidateQueries({queryKey: [EnvironmentVariableQueryKeys.ENVIRONMENT_VARIABLES, envId]})
        .catch(() => {
          // Ignore invalidation errors.
        });
      showToast(t('create.success', 'Environment variable created successfully'), 'success');
    },
    onError: () => {
      showToast(t('create.error', 'Failed to create the environment variable. Please try again.'), 'error');
    },
  });
}
