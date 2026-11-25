// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ProjectQueryKeys from '../constants/project-query-keys';

/** Deletes a project by ID, then refreshes the project list the header and the projects page show. */
export default function useDeleteProject(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      await http.request({
        url: `${getServerUrl()}/projects/${encodeURIComponent(id)}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [ProjectQueryKeys.PROJECTS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('projects:delete.success', 'Project deleted'), 'success');
    },
  });
}
