// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import NotificationTemplateQueryKeys from '../constants/notification-template-query-keys';
import type {NotificationChannel} from '../models/notification-template';
import type {TemplateListResponse} from '../models/responses';

/**
 * Parameters for the {@link useGetNotificationTemplates} hook.
 *
 * @public
 */
export interface UseGetNotificationTemplatesParams {
  /**
   * Maximum number of records to return.
   */
  limit?: number;
  /**
   * Number of records to skip for pagination.
   */
  offset?: number;
  /**
   * List only the templates of this project. Without it, every template is listed.
   */
  projectId?: string;
}

/**
 * Fetches a paginated list of notification templates for a channel from the server.
 *
 * @param channel - The notification channel (`email` or `sms`) to list templates for
 * @param params - Optional pagination parameters and the project to list
 * @returns TanStack Query result containing the template list, loading state, and error information
 *
 * @public
 */
export default function useGetNotificationTemplates(
  channel: NotificationChannel,
  params?: UseGetNotificationTemplatesParams,
): UseQueryResult<TemplateListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, projectId} = params ?? {};

  return useQuery<TemplateListResponse>({
    queryKey: [NotificationTemplateQueryKeys.TEMPLATES, channel, {limit, offset, ...(projectId ? {projectId} : {})}],
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<TemplateListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });
      if (projectId) {
        queryParams.append('projectId', projectId);
      }

      const response: {
        data: TemplateListResponse;
      } = await http.request({
        url: `${serverUrl}/notification-templates/${channel}/templates?${queryParams.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
