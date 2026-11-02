// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

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
