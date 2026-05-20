/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {useThunderID} from '@thunderid/react';
import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
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
