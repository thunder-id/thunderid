// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import NotificationTemplateQueryKeys from '../constants/notification-template-query-keys';
import type {NotificationChannel, NotificationTemplate} from '../models/notification-template';
import type {CreateTemplateRequest} from '../models/requests';

/**
 * Variables for the {@link useCreateNotificationTemplate} mutation.
 *
 * @public
 */
export interface CreateNotificationTemplateVariables {
  /**
   * The channel to create the template under.
   */
  channel: NotificationChannel;
  /**
   * The template data to create.
   */
  data: CreateTemplateRequest;
}

/**
 * Creates a new notification template on the server.
 *
 * On success, invalidates the channel's template list so the listing refetches.
 *
 * @returns TanStack Query mutation for creating a template
 *
 * @public
 */
export default function useCreateNotificationTemplate(): UseMutationResult<
  NotificationTemplate,
  Error,
  CreateNotificationTemplateVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('notificationTemplates');
  const {showToast} = useToast();

  return useMutation<NotificationTemplate, Error, CreateNotificationTemplateVariables>({
    mutationFn: async ({channel, data}: CreateNotificationTemplateVariables): Promise<NotificationTemplate> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: NotificationTemplate;
      } = await http.request({
        url: `${serverUrl}/notification-templates/${channel}/templates`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_data, {channel}) => {
      queryClient.invalidateQueries({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, channel]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('create.success', 'Template created successfully.'), 'success');
    },
  });
}
