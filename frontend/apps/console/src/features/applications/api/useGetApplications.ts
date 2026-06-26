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

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
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
  /**
   * Search term matched against the application name, client ID and description.
   */
  search?: string;
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
/**
 * Builds a SCIM-style filter expression that matches the search term (case-insensitive substring)
 * against the application name, client ID and description, OR'd together. Double quotes are stripped
 * since the backend filter grammar does not support escaped quotes inside quoted values.
 *
 * @param search - The raw search term.
 * @returns A SCIM filter expression, or an empty string when there is no term.
 */
function buildApplicationsFilter(search: string): string {
  const term: string = search.trim().replace(/"/g, '');
  if (!term) {
    return '';
  }
  return ['name', 'clientId', 'description'].map((field) => `${field} co "${term}"`).join(' OR ');
}

export default function useGetApplications(params?: UseGetApplicationsParams): UseQueryResult<ApplicationListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0, search = ''} = params ?? {};
  const appsFilter: string = buildApplicationsFilter(search);

  return useQuery<ApplicationListResponse>({
    queryKey: [ApplicationQueryKeys.APPLICATIONS, {limit, offset, filter: appsFilter}],
    queryFn: async (): Promise<ApplicationListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      if (appsFilter) {
        queryParams.set('filter', appsFilter);
      }

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
