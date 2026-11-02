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

// Server-side max page size; the backend caps limit at this value (MaxPageSize in
// backend/internal/system/constants), so the picker pages through with offset to load every
// template of the channel.
const TEMPLATE_PAGE_SIZE = 100;

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
    queryFn: async (): Promise<NotificationTemplateSummary[]> => {
      const serverUrl: string = getServerUrl();
      const templates: NotificationTemplateSummary[] = [];

      for (let offset = 0; ; offset += TEMPLATE_PAGE_SIZE) {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: TEMPLATE_PAGE_SIZE.toString(),
          offset: offset.toString(),
        });

        const response: {
          data: NotificationTemplateListResponse;
        } = await http.request({
          url: `${serverUrl}/notification-templates/${channel}/templates?${queryParams.toString()}`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        const page: NotificationTemplateSummary[] = response.data.templates;
        templates.push(...page);

        // A short page is always the last; it also ends the loop when totalResults is absent.
        if (page.length < TEMPLATE_PAGE_SIZE) {
          break;
        }

        // With a full page, only the reported total can tell us we are done; without it, keep
        // paging until a short/empty page arrives.
        const total: number | undefined = response.data.totalResults;
        if (total !== undefined && templates.length >= total) {
          break;
        }
      }

      return templates;
    },
  });
}
