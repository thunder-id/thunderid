// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import {PROJECTS_QUERY_KEY, type Project} from './useGetProjects';

/** The body of a create project request (`POST /projects`). */
export interface CreateProjectRequest {
  handle: string;
  displayName: string;
  description?: string;
}

/** Creates a project, then refreshes the project list the header and the projects page show. */
export default function useCreateProject(): UseMutationResult<Project, Error, CreateProjectRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<Project, Error, CreateProjectRequest>({
    mutationFn: async (data: CreateProjectRequest): Promise<Project> => {
      const response: {
        data: Project;
      } = await http.request({
        url: `${getServerUrl()}/projects`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [PROJECTS_QUERY_KEY]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('common:projects.create.success', 'Project created'), 'success');
    },
  });
}
