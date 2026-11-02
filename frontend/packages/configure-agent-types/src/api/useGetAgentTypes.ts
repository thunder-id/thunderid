// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AgentTypeQueryKeys from '../constants/agentTypeQueryKeys';
import type {AgentTypeListParams} from '../models/requests';
import type {AgentTypeListResponse} from '../models/responses';

/**
 * Custom React hook to fetch a paginated list of agent types from the server.
 *
 * @param params - Optional pagination parameters
 * @param params.limit - Maximum number of records to return
 * @param params.offset - Number of records to skip for pagination
 * @param options - Optional query options
 * @param options.enabled - Set to false to skip the request
 * @returns TanStack Query result object containing agent types list data, loading state, and error information
 */
export default function useGetAgentTypes(
  params?: AgentTypeListParams,
  options?: {enabled?: boolean},
): UseQueryResult<AgentTypeListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset} = params ?? {};

  return useQuery<AgentTypeListResponse>({
    queryKey: [AgentTypeQueryKeys.AGENT_TYPES, {limit, offset}],
    queryFn: async (): Promise<AgentTypeListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams();

      if (limit !== undefined) {
        queryParams.append('limit', limit.toString());
      }
      if (offset !== undefined) {
        queryParams.append('offset', offset.toString());
      }
      queryParams.append('include', 'display');

      const queryString: string = queryParams.toString();
      const url = `${serverUrl}/agent-types${queryString ? `?${queryString}` : ''}`;

      const response: {
        data: AgentTypeListResponse;
      } = await http.request({
        url,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: options?.enabled ?? true,
  });
}
