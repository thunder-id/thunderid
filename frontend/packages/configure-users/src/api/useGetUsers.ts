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
import type {ApiFilteringParams} from '@thunderid/types';
import UserQueryKeys from '../constants/user-query-keys';
import type {UserListResponse} from '../models/users';

/**
 * Custom hook to fetch a list of users.
 *
 * @param params - Optional query parameters for filtering and pagination
 * @returns TanStack Query result object containing user list data, loading state, and error information
 */
export default function useGetUsers(params?: ApiFilteringParams): UseQueryResult<UserListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset, filter} = params ?? {};

  return useQuery<UserListResponse>({
    queryKey: [UserQueryKeys.USERS, {limit, offset, filter}],
    queryFn: async (): Promise<UserListResponse> => {
      const serverUrl: string = getServerUrl();
      const searchParams: URLSearchParams = new URLSearchParams();

      if (limit !== undefined) {
        searchParams.append('limit', String(limit));
      }
      if (offset !== undefined) {
        searchParams.append('offset', String(offset));
      }
      if (filter) {
        searchParams.append('filter', filter);
      }
      searchParams.append('include', 'display');

      const queryString: string = searchParams.toString();

      const response: {
        data: UserListResponse;
      } = await http.request({
        url: `${serverUrl}/users${queryString ? `?${queryString}` : ''}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
