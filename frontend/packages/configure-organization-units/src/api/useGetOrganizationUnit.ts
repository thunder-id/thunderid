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
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import type {OrganizationUnit} from '../models/organization-unit';

/**
 * Custom React hook to fetch a single organization unit by its ID from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates.
 *
 * @param id - The unique identifier of the organization unit to fetch
 * @param enabled - Whether the query should be enabled (default: true when id is provided)
 * @returns TanStack Query result object containing organization unit data, loading state, and error information
 *
 * @example
 * ```tsx
 * function OrganizationUnitDetails({ id }: { id: string }) {
 *   const { data, isLoading, error } = useGetOrganizationUnit(id);
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return <div>{data?.name}</div>;
 * }
 * ```
 */
export default function useGetOrganizationUnit(
  id: string | undefined,
  enabled = true,
): UseQueryResult<OrganizationUnit> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<OrganizationUnit>({
    queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNIT, id],
    queryFn: async (): Promise<OrganizationUnit> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: OrganizationUnit;
      } = await http.request({
        url: `${serverUrl}/organization-units/${encodeURIComponent(id!)}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: enabled && Boolean(id),
  });
}
