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
import type {User} from '@thunderid/types';
import UserQueryKeys from '../constants/user-query-keys';

/**
 * Custom hook to fetch a single user by ID.
 *
 * @param userId - The ID of the user to fetch
 * @returns TanStack Query result object containing user data, loading state, and error information
 */
export default function useGetUser(userId: string | undefined): UseQueryResult<User> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<User>({
    queryKey: [UserQueryKeys.USER, userId],
    queryFn: async (): Promise<User> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: User;
      } = await http.request({
        url: `${serverUrl}/users/${userId}?include=display`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(userId),
  });
}
