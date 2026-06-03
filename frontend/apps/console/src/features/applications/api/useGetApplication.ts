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
import type {Application} from '../models/application';

/**
 * Custom React hook to fetch a single application by ID from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates. The query is automatically disabled
 * when no applicationId is provided.
 *
 * @param applicationId - The unique identifier of the application to fetch
 * @returns TanStack Query result object containing application data, loading state, and error information
 *
 * @example
 * ```tsx
 * function ApplicationDetails({ id }: { id: string }) {
 *   const { data, isLoading, error } = useGetApplication(id);
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *   if (!data) return <div>Not found</div>;
 *
 *   return (
 *     <div>
 *       <h1>{data.name}</h1>
 *       <p>{data.description}</p>
 *     </div>
 *   );
 * }
 * ```
 *
 * @public
 */
export default function useGetApplication(applicationId: string): UseQueryResult<Application> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Application>({
    queryKey: [ApplicationQueryKeys.APPLICATION, applicationId],
    queryFn: async (): Promise<Application> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: Application;
      } = await http.request({
        url: `${serverUrl}/applications/${applicationId}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(applicationId),
  });
}
