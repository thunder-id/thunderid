// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SettingsQueryKeys from '../constants/settings-query-keys';
import type {ServerConfigLayers} from '../models/responses';

/**
 * Variables for the {@link useUpdateServerConfigSection} mutation.
 *
 * @public
 */
export interface UpdateServerConfigSectionVariables<T> {
  /**
   * The writable value to persist. It replaces the writable layer whole and a field it leaves out
   * returns to its default, so it carries every field that stays set, not only this save's changes.
   */
  data: T;
}

/**
 * Replaces the writable layer of one server-config section.
 *
 * @param name - The section name, such as `session`
 * @param successMessage - The confirmation shown once the section is saved
 * @returns TanStack Query mutation for writing the section.
 *
 * @public
 */
export default function useUpdateServerConfigSection<T>(
  name: string,
  successMessage: string,
): UseMutationResult<ServerConfigLayers<T>, Error, UpdateServerConfigSectionVariables<T>> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {showToast} = useToast();

  return useMutation<ServerConfigLayers<T>, Error, UpdateServerConfigSectionVariables<T>>({
    mutationFn: async ({data}: UpdateServerConfigSectionVariables<T>): Promise<ServerConfigLayers<T>> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ServerConfigLayers<T>;
      } = await http.request({
        url: `${serverUrl}/server-config/${name}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: async (section) => {
      // A read of the section still in flight would land after this and put the old layers back, so
      // it is cancelled first.
      await queryClient.cancelQueries({queryKey: [SettingsQueryKeys.SERVER_CONFIG, name]});
      // PUT returns the recomputed layers, so the card shows them without a refetch.
      queryClient.setQueryData([SettingsQueryKeys.SERVER_CONFIG, name], section);
      showToast(successMessage, 'success');
    },
  });
}
