/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import FlowQueryKeys from '../constants/flow-query-keys';
import type {FlowUsagesResponse} from '../models/responses';

/**
 * Custom hook to fetch resources that reference a flow.
 * Used to populate the pre-delete confirmation dialog.
 *
 * @param flowId - The unique identifier of the flow
 * @param enabled - Whether the query should run (default true)
 * @returns TanStack Query result with flow usages data
 */
export default function useGetFlowUsages(flowId: string | null, enabled = true): UseQueryResult<FlowUsagesResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<FlowUsagesResponse>({
    queryKey: [FlowQueryKeys.FLOW_USAGES, flowId],
    queryFn: async (): Promise<FlowUsagesResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {data: FlowUsagesResponse} = await http.request({
        url: `${serverUrl}/flows/${encodeURIComponent(flowId!)}/usages`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(flowId) && enabled,
  });
}
