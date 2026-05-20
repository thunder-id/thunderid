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
import fetchOrganizationUnits from './fetchOrganizationUnits';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import type {OrganizationUnitListParams} from '../models/requests';
import type {OrganizationUnitListResponse} from '../models/responses';

/**
 * Custom React hook to fetch a paginated list of organization units from the server.
 *
 * This hook uses TanStack Query to manage the server state and provides automatic
 * caching, refetching, and background updates. The query is keyed by the pagination
 * parameters to ensure proper cache management.
 *
 * @param params - Optional pagination parameters
 * @param params.limit - Maximum number of records to return (default: 30)
 * @param params.offset - Number of records to skip for pagination (default: 0)
 * @returns TanStack Query result object containing organization units list data, loading state, and error information
 *
 * @example
 * ```tsx
 * function OrganizationUnitsList() {
 *   const { data, isLoading, error } = useGetOrganizationUnits();
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.organizationUnits.map((ou) => (
 *         <li key={ou.id}>{ou.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useGetOrganizationUnits(
  params?: OrganizationUnitListParams,
  enabled = true,
): UseQueryResult<OrganizationUnitListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<OrganizationUnitListResponse>({
    queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNITS, {limit, offset}],
    queryFn: async (): Promise<OrganizationUnitListResponse> =>
      fetchOrganizationUnits(http, getServerUrl(), {limit, offset}),
    enabled,
  });
}
