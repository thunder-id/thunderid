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

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import RoleQueryKeys from '../constants/role-query-keys';
import type {RoleAssignmentListParams} from '../models/requests';
import type {RoleAssignmentListResponse} from '../models/role';

export interface UseGetRoleAssignmentsParams extends RoleAssignmentListParams {
  roleId: string;
  /** Whether the query should be enabled (default: true when roleId is provided) */
  enabled?: boolean;
}

/**
 * Custom React hook to fetch a paginated list of role assignments.
 *
 * @param params - Role ID and optional pagination parameters
 * @returns TanStack Query result object containing assignments list data
 */
export default function useGetRoleAssignments(
  params: UseGetRoleAssignmentsParams,
): UseQueryResult<RoleAssignmentListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {roleId, limit = 30, offset = 0, include, type, enabled = true} = params;

  return useQuery<RoleAssignmentListResponse>({
    queryKey: [RoleQueryKeys.ROLE_ASSIGNMENTS, roleId, {limit, offset, include, type}],
    queryFn: async (): Promise<RoleAssignmentListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
      });
      if (include) {
        queryParams.set('include', include);
      }
      if (type) {
        queryParams.set('type', type);
      }

      const response: {data: RoleAssignmentListResponse} = await http.request({
        url: `${serverUrl}/roles/${roleId}/assignments?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: enabled && !!roleId,
  });
}
