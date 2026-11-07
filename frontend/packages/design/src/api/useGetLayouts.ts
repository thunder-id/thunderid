// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import DesignQueryKeys from '../constants/design-query-keys';
import type {LayoutListResponse} from '../models/responses';

interface UseGetLayoutsParams {
  limit?: number;
  offset?: number;
  /** List only the layouts of this project. Without it, every layout is listed. */
  projectId?: string;
}

/**
 * Custom hook to fetch the list of layout configurations from the server.
 *
 * @param params - Optional query parameters
 * @param params.limit - Maximum number of records to return (default: 30)
 * @param params.offset - Number of records to skip for pagination (default: 0)
 * @param params.projectId - List only the layouts of this project
 * @returns TanStack Query result object with layout list data
 */
export default function useGetLayouts(params?: UseGetLayoutsParams): UseQueryResult<LayoutListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, projectId} = params ?? {};

  return useQuery<LayoutListResponse>({
    queryKey: [DesignQueryKeys.LAYOUTS, {limit, offset, ...(projectId ? {projectId} : {})}],
    queryFn: async (): Promise<LayoutListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });
      if (projectId) {
        queryParams.append('projectId', projectId);
      }

      const response: {
        data: LayoutListResponse;
      } = await http.request({
        url: `${serverUrl}/design/layouts?${queryParams.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
