// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import type {NotificationConfigResponse, NotificationConfigValue} from '../models/connection';

// Updates the writable layer of the notification server-config section.
export default function useSetDefaultEmailSender(): UseMutationResult<
  NotificationConfigResponse,
  Error,
  NotificationConfigValue
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<NotificationConfigResponse, Error, NotificationConfigValue>({
    mutationFn: async (data): Promise<NotificationConfigResponse> => {
      const serverUrl = getServerUrl();

      const response: {data: NotificationConfigResponse} = await http.request({
        url: `${serverUrl}/server-config/${ConnectionQueryKeys.NOTIFICATION_CONFIG}`,
        method: 'PUT',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (data) => {
      // PUT returns the recomputed layers, so keep the read model in sync without a refetch.
      queryClient.setQueryData([ConnectionQueryKeys.SERVER_CONFIG, ConnectionQueryKeys.NOTIFICATION_CONFIG], data);
    },
  });
}
