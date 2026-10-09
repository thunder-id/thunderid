// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import NotificationTemplateQueryKeys from '../constants/notification-template-query-keys';
import type {NotificationChannel, NotificationTemplate} from '../models/notification-template';
import type {UpdateTemplateRequest} from '../models/requests';

/**
 * Variables for the {@link useUpdateNotificationTemplate} mutation.
 *
 * @public
 */
export interface UpdateNotificationTemplateVariables {
  /**
   * The channel the template belongs to.
   */
  channel: NotificationChannel;
  /**
   * The unique identifier of the template to update.
   */
  id: string;
  /**
   * The updated template data.
   */
  data: UpdateTemplateRequest;
}

/**
 * Updates an existing notification template on the server.
 *
 * On success, invalidates both the single-template query and the channel's list.
 *
 * @returns TanStack Query mutation for updating a template
 *
 * @public
 */
export default function useUpdateNotificationTemplate(): UseMutationResult<
  NotificationTemplate,
  Error,
  UpdateNotificationTemplateVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('notificationTemplates');
  const {showToast} = useToast();

  return useMutation<NotificationTemplate, Error, UpdateNotificationTemplateVariables>({
    mutationFn: async ({channel, id, data}: UpdateNotificationTemplateVariables): Promise<NotificationTemplate> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: NotificationTemplate;
      } = await http.request({
        url: `${serverUrl}/notification-templates/${channel}/templates/${id}`,
        method: 'PUT',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, {channel, id}) => {
      queryClient
        .invalidateQueries({queryKey: [NotificationTemplateQueryKeys.TEMPLATE, channel, id]})
        .catch(() => {
          // Ignore invalidation errors
        });
      queryClient.invalidateQueries({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, channel]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('update.success', 'Template updated successfully.'), 'success');
    },
  });
}
