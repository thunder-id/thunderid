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
import type {GroupListResponse} from '../models/group';
import type {OrganizationUnitListParams} from '../models/requests';

/**
 * Custom React hook to fetch groups belonging to a specific organization unit.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates.
 *
 * @param organizationUnitId - The ID of the organization unit
 * @param params - Optional pagination parameters
 * @param params.limit - Maximum number of records to return (default: 30)
 * @param params.offset - Number of records to skip for pagination (default: 0)
 * @returns TanStack Query result object containing groups list data
 *
 * @example
 * ```tsx
 * function OUGroupsList({ ouId }: { ouId: string }) {
 *   const { data, isLoading, error } = useGetOrganizationUnitGroups(ouId);
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.groups.map((group) => (
 *         <li key={group.id}>{group.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useGetOrganizationUnitGroups(
  organizationUnitId: string | undefined,
  params?: OrganizationUnitListParams,
): UseQueryResult<GroupListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<GroupListResponse>({
    queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNIT_GROUPS, organizationUnitId, {limit, offset}],
    queryFn: async (): Promise<GroupListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });

      const response: {
        data: GroupListResponse;
      } = await http.request({
        url: `${serverUrl}/organization-units/${organizationUnitId}/groups?${queryParams.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(organizationUnitId),
  });
}
