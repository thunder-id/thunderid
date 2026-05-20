/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
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

import type {OrganizationUnitListResponse} from '../models/responses';

/**
 * Fetches a paginated list of root organization units from the server.
 *
 * This is a standalone API utility that can be used both by React Query hooks
 * and by imperative fetch calls (e.g. via queryClient.fetchQuery).
 *
 * @param http - The HTTP client from useThunderID
 * @param serverUrl - The base server URL
 * @param params - Pagination parameters
 * @param params.limit - Maximum number of records to return
 * @param params.offset - Number of records to skip
 * @returns The organization unit list response
 */
export default async function fetchOrganizationUnits(
  http: {request: (...args: never[]) => Promise<{data: OrganizationUnitListResponse}>},
  serverUrl: string,
  params: {limit: number; offset: number},
): Promise<OrganizationUnitListResponse> {
  const queryParams = new URLSearchParams({
    limit: String(params.limit),
    offset: String(params.offset),
  });

  const response: {data: OrganizationUnitListResponse} = await http.request({
    url: `${serverUrl}/organization-units?${queryParams.toString()}`,
    method: 'GET',
    headers: {'Content-Type': 'application/json'},
  } as never);

  return response.data;
}
