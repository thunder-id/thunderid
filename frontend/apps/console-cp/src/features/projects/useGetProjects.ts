// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';

/** A project, as the project API lists it. */
export interface Project {
  id: string;
  handle: string;
  displayName: string;
  description?: string;
  rootOuId: string;
  createdAt: string;
}

/** A page of projects, oldest first. */
export interface ProjectListResponse {
  totalResults: number;
  startIndex: number;
  count: number;
  projects: Project[];
}

export const PROJECTS_QUERY_KEY = 'projects';

/**
 * The deployment's projects, read from the project API (`GET /projects`). The header only shows
 * them, so the list is fetched once and a failure is not retried.
 */
export default function useGetProjects(): UseQueryResult<ProjectListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ProjectListResponse>({
    queryKey: [PROJECTS_QUERY_KEY],
    staleTime: Infinity,
    retry: false,
    queryFn: async (): Promise<ProjectListResponse> => {
      const response: {
        data: ProjectListResponse;
      } = await http.request({
        url: `${getServerUrl()}/projects`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
