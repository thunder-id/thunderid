// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ProjectQueryKeys from '../constants/project-query-keys';
import type {Project, ProjectListResponse} from '../models/project';

/** The largest page the project API returns (`limit` maximum in `api/project.yaml`). */
const PAGE_SIZE = 100;

/**
 * The deployment's projects, read from the project API (`GET /projects`). Every page is loaded,
 * since the header, sidebar and projects page all work from the full list. The list is fetched
 * once and a failure is not retried.
 */
export default function useGetProjects(): UseQueryResult<ProjectListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ProjectListResponse>({
    queryKey: [ProjectQueryKeys.PROJECTS],
    staleTime: Infinity,
    retry: false,
    queryFn: async (): Promise<ProjectListResponse> => {
      const projects: Project[] = [];
      let totalResults = 0;

      do {
        // Each page starts where the previous one ended, so the pages are fetched in turn.
        const response: {
          data: ProjectListResponse;
        } = await http.request({
          url: `${getServerUrl()}/projects?limit=${PAGE_SIZE}&offset=${projects.length}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        totalResults = response.data.totalResults;
        projects.push(...response.data.projects);

        if (response.data.projects.length === 0) break;
      } while (projects.length < totalResults);

      return {totalResults, startIndex: 1, count: projects.length, projects};
    },
  });
}
