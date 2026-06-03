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
import UserTypeQueryKeys from '../constants/userTypeQueryKeys';
import type {UserTypeListParams, UserTypeListResponse} from '../types/user-types';

/**
 * Custom React hook to fetch a paginated list of user types from the server.
 *
 * @param params - Optional pagination parameters
 * @param params.limit - Maximum number of records to return
 * @param params.offset - Number of records to skip for pagination
 * @returns TanStack Query result object containing user types list data, loading state, and error information
 */
export default function useGetUserTypes(params?: UserTypeListParams): UseQueryResult<UserTypeListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset} = params ?? {};

  return useQuery<UserTypeListResponse>({
    queryKey: [UserTypeQueryKeys.USER_TYPES, {limit, offset}],
    queryFn: async (): Promise<UserTypeListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams();

      if (limit !== undefined) {
        queryParams.append('limit', limit.toString());
      }
      if (offset !== undefined) {
        queryParams.append('offset', offset.toString());
      }
      queryParams.append('include', 'display');

      const queryString: string = queryParams.toString();
      const url = `${serverUrl}/user-types${queryString ? `?${queryString}` : ''}`;

      const response: {
        data: UserTypeListResponse;
      } = await http.request({
        url,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
