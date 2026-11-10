// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useCapabilitiesContext, useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
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
 * @param params.projectId - List only the organization units of this project
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
  const {confinedOuId} = useCapabilitiesContext();
  const {limit = 30, offset = 0, projectId} = params ?? {};

  return useQuery<OrganizationUnitListResponse>({
    queryKey: [
      OrganizationUnitQueryKeys.ORGANIZATION_UNITS,
      {limit, offset, ...(projectId ? {projectId} : {}), ...(confinedOuId ? {confinedOuId} : {})},
    ],
    queryFn: async (): Promise<OrganizationUnitListResponse> =>
      fetchOrganizationUnits(http, getServerUrl(), {limit, offset, projectId, confinedOuId}),
    enabled,
  });
}
