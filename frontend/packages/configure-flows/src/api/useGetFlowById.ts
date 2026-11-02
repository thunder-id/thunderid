// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import FlowQueryKeys from '../constants/flow-query-keys';
import type {FlowDefinitionResponse} from '../models/responses';

/**
 * Custom React hook to fetch a single flow by its ID from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates.
 *
 * @param flowId - The unique identifier of the flow to fetch
 * @param enabled - Whether the query should be enabled (default: true when flowId is provided)
 * @returns TanStack Query result object containing flow data, loading state, and error information
 *
 * @example
 * ```tsx
 * function FlowEditor({ flowId }: { flowId: string }) {
 *   const { data, isLoading, error } = useGetFlowById(flowId);
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return <FlowCanvas flow={data} />;
 * }
 * ```
 */
export default function useGetFlowById(
  flowId: string | undefined,
  enabled = true,
): UseQueryResult<FlowDefinitionResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<FlowDefinitionResponse>({
    queryKey: [FlowQueryKeys.FLOW, flowId],
    queryFn: async (): Promise<FlowDefinitionResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: FlowDefinitionResponse;
      } = await http.request({
        url: `${serverUrl}/flows/${flowId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: enabled && Boolean(flowId),
  });
}
