// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import NotificationTemplateQueryKeys from '../constants/notification-template-query-keys';
import type {NotificationChannel, NotificationTemplate} from '../models/notification-template';

/**
 * Fetches a single notification template by id from the server.
 *
 * The query is automatically disabled when no id is provided.
 *
 * @param channel - The notification channel (`email` or `sms`) the template belongs to
 * @param id - The unique identifier of the template to fetch
 * @returns TanStack Query result containing the template, loading state, and error information
 *
 * @public
 */
export default function useGetNotificationTemplate(
  channel: NotificationChannel,
  id: string,
): UseQueryResult<NotificationTemplate> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<NotificationTemplate>({
    queryKey: [NotificationTemplateQueryKeys.TEMPLATE, channel, id],
    queryFn: async (): Promise<NotificationTemplate> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: NotificationTemplate;
      } = await http.request({
        url: `${serverUrl}/notification-templates/${channel}/templates/${id}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(id),
  });
}
