// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ProjectQueryKeys from '../constants/project-query-keys';
import type {Project, ProjectListResponse} from '../models/project';
import type {CreateProjectRequest} from '../models/requests';

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
    onSuccess: (project) => {
      // The caller selects the new project straight away, so it has to be in the list before the
      // refetch lands. Otherwise the selection falls back to the first project in the meantime.
      queryClient.setQueryData<ProjectListResponse>([ProjectQueryKeys.PROJECTS], (current) => {
        const list = current ?? {totalResults: 0, startIndex: 1, count: 0, projects: []};
        if (list.projects.some((item) => item.id === project.id)) {
          return list;
        }
        return {
          ...list,
          totalResults: list.totalResults + 1,
          count: list.count + 1,
          projects: [...list.projects, project],
        };
      });
      queryClient.invalidateQueries({queryKey: [ProjectQueryKeys.PROJECTS]}).catch(() => {
        // Ignore invalidation errors
      });
      showToast(t('projects:create.success', 'Project created'), 'success');
    },
  });
}
