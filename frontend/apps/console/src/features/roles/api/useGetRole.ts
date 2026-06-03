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
import type {Role} from '../models/role';

/**
 * Custom React hook to fetch a single role by ID.
 *
 * @param roleId - The role ID to fetch
 * @returns TanStack Query result object containing role data
 */
export default function useGetRole(roleId: string): UseQueryResult<Role> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Role>({
    queryKey: [RoleQueryKeys.ROLE, roleId],
    queryFn: async (): Promise<Role> => {
      const serverUrl: string = getServerUrl();

      const response: {data: Role} = await http.request({
        url: `${serverUrl}/roles/${roleId}?include=display`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: !!roleId,
  });
}
