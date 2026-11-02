// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import type {NotificationConfigResponse} from '../models/connection';

export interface UseGetNotificationConfigOptions {
  enabled?: boolean;
}

// Fetches the notification server-config section, which names the default email sender.
export default function useGetNotificationConfig(
  options?: UseGetNotificationConfigOptions,
): UseQueryResult<NotificationConfigResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<NotificationConfigResponse>({
    queryKey: [ConnectionQueryKeys.SERVER_CONFIG, ConnectionQueryKeys.NOTIFICATION_CONFIG],
    enabled: options?.enabled ?? true,
    queryFn: async (): Promise<NotificationConfigResponse> => {
      const serverUrl = getServerUrl();

      const response: {data: NotificationConfigResponse} = await http.request({
        url: `${serverUrl}/server-config/${ConnectionQueryKeys.NOTIFICATION_CONFIG}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
