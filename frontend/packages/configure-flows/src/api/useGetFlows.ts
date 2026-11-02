// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import FlowQueryKeys from '../constants/flow-query-keys';
import type {FlowType} from '../models/flows';
import type {FlowListResponse} from '../models/responses';

/**
 * Parameters for the {@link useGetFlows} hook.
 */
export interface UseGetFlowsParams {
  /**
   * Whether to run the query.
   */
  enabled?: boolean;
  /**
   * Filter by flow type.
   */
  flowType?: FlowType;
  /**
   * Maximum number of records to return (default: 30, max: 100)
   */
  limit?: number;
  /**
   * Number of records to skip for pagination (default: 0)
   */
  offset?: number;
  /**
   * Fetch every page matching the filters instead of returning only one page.
   */
  fetchAllPages?: boolean;
}

/**
 * Custom React hook to fetch a paginated list of flows from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates. The query is keyed by the pagination
 * and filter parameters to ensure proper cache management.
 *
 * @param params - Optional pagination and filtering parameters
 * @param params.flowType - Filter flows by type (AUTHENTICATION or REGISTRATION)
 * @param params.limit - Maximum number of records to return (default: 30, max: 100)
 * @param params.offset - Number of records to skip for pagination (default: 0)
 * @param params.fetchAllPages - Whether to fetch and combine every matching page
 * @returns TanStack Query result object containing flows list data, loading state, and error information
 *
 * @example
 * ```tsx
 * // Fetch all flows with default pagination
 * function FlowsList() {
 *   const { data, isLoading, error } = useGetFlows();
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.flows.map((flow) => (
 *         <li key={flow.id}>{flow.name} ({flow.flowType})</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 *
 * @example
 * ```tsx
 * // Fetch only authentication flows with custom pagination
 * function AuthFlowsList() {
 *   const { data, isLoading, error } = useGetFlows({
 *     flowType: FlowType.AUTHENTICATION,
 *     limit: 10,
 *     offset: 0
 *   });
 *
 *   if (isLoading) return <div>Loading authentication flows...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <div>
 *       <h2>Authentication Flows ({data?.totalResults} total)</h2>
 *       <ul>
 *         {data?.flows.map((flow) => (
 *           <li key={flow.id}>
 *             {flow.name} - Version {flow.activeVersion}
 *           </li>
 *         ))}
 *       </ul>
 *     </div>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useGetFlows(params?: UseGetFlowsParams): UseQueryResult<FlowListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {flowType, limit = 30, offset = 0, enabled = true, fetchAllPages = false} = params ?? {};

  return useQuery<FlowListResponse>({
    queryKey: [FlowQueryKeys.FLOWS, {flowType, limit, offset, ...(fetchAllPages ? {fetchAllPages} : {})}],
    enabled,
    queryFn: async (): Promise<FlowListResponse> => {
      const serverUrl: string = getServerUrl();
      const fetchPage = async (pageOffset: number): Promise<FlowListResponse> => {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: limit.toString(),
          offset: pageOffset.toString(),
        });

        if (flowType) {
          queryParams.append('flowType', flowType);
        }

        const response: {
          data: FlowListResponse;
        } = await http.request({
          url: `${serverUrl}/flows?${queryParams.toString()}`,
          method: 'GET',
          headers: {
            'Content-Type': 'application/json',
          },
        } as unknown as Parameters<typeof http.request>[0]);

        return response.data;
      };

      if (!fetchAllPages) {
        return fetchPage(offset);
      }

      const firstPage = await fetchPage(offset);
      const flows: FlowListResponse['flows'] = [...firstPage.flows];
      const totalResults = firstPage.totalResults;

      for (let pageOffset = offset + limit; flows.length < totalResults; pageOffset += limit) {
        const page = await fetchPage(pageOffset);
        flows.push(...page.flows);

        if (page.count < limit || page.flows.length < limit) {
          break;
        }
      }

      return {
        ...firstPage,
        totalResults,
        startIndex: offset + 1,
        count: flows.length,
        flows,
      };
    },
  });
}
