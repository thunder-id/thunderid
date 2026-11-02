// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import NotificationTemplateQueryKeys from '../constants/notification-template-query-keys';
import type {NotificationChannel} from '../models/notification-template';

/**
 * Variables for the {@link useDeleteNotificationTemplate} mutation.
 *
 * @public
 */
export interface DeleteNotificationTemplateVariables {
  /**
   * The channel the template belongs to.
   */
  channel: NotificationChannel;
  /**
   * The unique identifier of the template to delete.
   */
  id: string;
}

/**
 * Deletes a notification template from the server.
 *
 * On success, removes the single-template query from cache and invalidates the channel's list.
 *
 * @returns TanStack Query mutation for deleting a template
 *
 * @public
 */
export default function useDeleteNotificationTemplate(): UseMutationResult<
  void,
  Error,
  DeleteNotificationTemplateVariables
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('notificationTemplates');
  const {showToast} = useToast();

  return useMutation<void, Error, DeleteNotificationTemplateVariables>({
    mutationFn: async ({channel, id}: DeleteNotificationTemplateVariables): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/notification-templates/${channel}/templates/${id}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, {channel, id}) => {
      queryClient.removeQueries({queryKey: [NotificationTemplateQueryKeys.TEMPLATE, channel, id]});
      queryClient.invalidateQueries({queryKey: [NotificationTemplateQueryKeys.TEMPLATES, channel]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('delete.success', 'Template deleted successfully.'), 'success');
    },
  });
}
