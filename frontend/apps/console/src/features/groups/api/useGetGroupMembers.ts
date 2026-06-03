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
import type {MemberListResponse} from '../models/group';
import type {GroupListParams} from '../models/requests';

/**
 * Custom React hook to fetch members of a specific group.
 *
 * @param groupId - The ID of the group
 * @param params - Optional pagination parameters
 * @returns TanStack Query result object containing group members data
 */
export default function useGetGroupMembers(
  groupId: string | undefined,
  params?: GroupListParams,
): UseQueryResult<MemberListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 30, offset = 0} = params ?? {};

  return useQuery<MemberListResponse>({
    queryKey: [GroupQueryKeys.GROUP_MEMBERS, groupId, {limit, offset}],
    queryFn: async (): Promise<MemberListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams: URLSearchParams = new URLSearchParams({
        limit: limit.toString(),
        offset: offset.toString(),
        include: 'display',
      });

      const response: {
        data: MemberListResponse;
      } = await http.request({
        url: `${serverUrl}/groups/${groupId}/members?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(groupId),
  });
}
