// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {OrganizationUnitListResponse} from '../models/responses';

/**
 * Fetches a paginated list of root organization units from the server, or of the top organization units of a
 * project when a project is given.
 *
 * This is a standalone API utility that can be used both by React Query hooks
 * and by imperative fetch calls (e.g. via queryClient.fetchQuery).
 *
 * @param http - The HTTP client from useThunderID
 * @param serverUrl - The base server URL
 * @param params - Pagination parameters
 * @param params.limit - Maximum number of records to return
 * @param params.offset - Number of records to skip
 * @param params.projectId - List only the organization units of this project
 * @returns The organization unit list response
 */
export default async function fetchOrganizationUnits(
  http: {request: (...args: never[]) => Promise<{data: OrganizationUnitListResponse}>},
  serverUrl: string,
  params: {limit: number; offset: number; projectId?: string},
): Promise<OrganizationUnitListResponse> {
  const queryParams = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });
  if (params.projectId) {
    queryParams.append('projectId', params.projectId);
  }

  const response: {data: OrganizationUnitListResponse} = await http.request({
    url: `${serverUrl}/organization-units?${queryParams.toString()}`,
    method: 'GET',
    headers: {'Content-Type': 'application/json'},
  } as never);

  return response.data;
}
