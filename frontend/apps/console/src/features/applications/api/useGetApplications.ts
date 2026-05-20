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
import ApplicationQueryKeys from '../constants/application-query-keys';
import type {ApplicationListResponse} from '../models/responses';

/**
 * Parameters for the {@link useGetApplications} hook.
 *
 * @public
 */
export interface UseGetApplicationsParams {
  /**
   * Maximum number of records to return.
   */
  limit?: number;
  /**
   * Number of records to skip for pagination.
   */
  offset?: number;
}

/**
 * Custom React hook to fetch a paginated list of applications from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates. The query is keyed by the pagination
 * parameters to ensure proper cache management.
 *
 * @param params - Optional pagination parameters
 * @param params.limit - Maximum number of records to return (default: 30)
 * @param params.offset - Number of records to skip for pagination (default: 0)
 * @returns TanStack Query result object containing applications list data, loading state, and error information
 *
 * @example
 * ```tsx
 * function ApplicationsList() {
 *   const { data, isLoading, error } = useGetApplications({ limit: 10, offset: 0 });
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.applications.map((app) => (
 *         <li key={app.id}>{app.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useGetApplications(params?: UseGetApplicationsParams): UseQueryResult<ApplicationListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<ApplicationListResponse>({
    queryKey: [ApplicationQueryKeys.APPLICATIONS, {limit, offset}],
    queryFn: async (): Promise<ApplicationListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      const response: {
        data: ApplicationListResponse;
      } = await http.request({
        url: `${serverUrl}/applications?${queryParams.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
