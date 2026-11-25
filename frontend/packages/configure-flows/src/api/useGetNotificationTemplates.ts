// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import FlowQueryKeys from '../constants/flow-query-keys';
import type {
  NotificationTemplateChannel,
  NotificationTemplateListResponse,
  NotificationTemplateSummary,
} from '../models/notification-templates';
import fetchAllPages from '../utils/fetchAllPages';

/**
 * Fetches all notification templates of a channel, used to populate the Email/SMS executor
 * template pickers. Backed by GET /notification-templates/{channel}/templates, paging through
 * with offset until the full set is loaded.
 *
 * @param channel - The template channel to list (email or sms).
 * @returns TanStack Query result with the channel's template summaries.
 *
 * @example
 * ```tsx
 * const {data, isLoading} = useGetNotificationTemplates('email');
 * ```
 */
export default function useGetNotificationTemplates(
  channel: NotificationTemplateChannel,
): UseQueryResult<NotificationTemplateSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<NotificationTemplateSummary[]>({
    queryKey: [FlowQueryKeys.NOTIFICATION_TEMPLATES, channel],
    queryFn: (): Promise<NotificationTemplateSummary[]> =>
      fetchAllPages<NotificationTemplateSummary>(async (limit, offset) => {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: limit.toString(),
          offset: offset.toString(),
        });

        const response: {
          data: NotificationTemplateListResponse;
        } = await http.request({
          url: `${getServerUrl()}/notification-templates/${channel}/templates?${queryParams.toString()}`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        return {items: response.data.templates, totalResults: response.data.totalResults};
      }),
  });
}
