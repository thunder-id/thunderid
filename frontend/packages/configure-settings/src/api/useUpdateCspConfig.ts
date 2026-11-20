// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {CspConfigResponse, CspValue} from '../models/responses';

/**
 * Variables for the {@link useUpdateCspConfig} mutation.
 *
 * @public
 */
export interface UpdateCspConfigVariables {
  /**
   * The writable CSP value to persist.
   */
  data: CspValue;
}

/**
 * Updates the writable layer of the CSP server-config section.
 *
 * @returns TanStack Query mutation for updating the content security policy.
 *
 * @public
 */
export default function useUpdateCspConfig(): UseMutationResult<CspConfigResponse, Error, UpdateCspConfigVariables> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<CspConfigResponse, Error, UpdateCspConfigVariables>({
    mutationFn: async ({data}: UpdateCspConfigVariables): Promise<CspConfigResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: CspConfigResponse;
      } = await http.request({
        url: `${serverUrl}/server-config/csp`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: async (data) => {
      const queryKey = [SettingsQueryKeys.SERVER_CONFIG, SettingsQueryKeys.CSP];
      // A refetch still in flight would otherwise overwrite the saved value with older data.
      await queryClient.cancelQueries({queryKey});
      // PUT returns the full config, so keep the read model in sync without a refetch.
      queryClient.setQueryData(queryKey, data);
      showToast(t('settings:csp.save.success', 'Content security policy updated.'), 'success');
    },
  });
}
