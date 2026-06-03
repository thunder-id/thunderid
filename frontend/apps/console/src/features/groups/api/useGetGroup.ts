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
import GroupQueryKeys from '../constants/group-query-keys';
import type {Group} from '../models/group';

/**
 * Custom React hook to fetch a single group by ID.
 *
 * @param groupId - The ID of the group to fetch
 * @returns TanStack Query result object containing group data
 */
export default function useGetGroup(groupId: string): UseQueryResult<Group> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Group>({
    queryKey: [GroupQueryKeys.GROUP, groupId],
    queryFn: async (): Promise<Group> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: Group;
      } = await http.request({
        url: `${serverUrl}/groups/${groupId}?include=display`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(groupId),
  });
}
