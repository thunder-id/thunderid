// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GroupQueryKeys from '../constants/group-query-keys';
import type {Member, MemberListResponse} from '../models/group';

const pageSize = 100;

export default function useGetAllGroupMembers(groupId: string | undefined): UseQueryResult<MemberListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<MemberListResponse>({
    queryKey: [GroupQueryKeys.GROUP_MEMBERS, groupId, 'all'],
    queryFn: async (): Promise<MemberListResponse> => {
      const serverUrl: string = getServerUrl();
      const members: Member[] = [];
      let totalResults = 0;
      let offset = 0;

      do {
        const queryParams: URLSearchParams = new URLSearchParams({
          limit: pageSize.toString(),
          offset: offset.toString(),
          include: 'display',
        });
        const response: {data: MemberListResponse} = await http.request({
          url: `${serverUrl}/groups/${groupId}/members?${queryParams.toString()}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);
        const page = response.data;

        members.push(...page.members);
        totalResults = page.totalResults;
        offset += page.count;

        if (page.count === 0) break;
      } while (offset < totalResults);

      return {
        totalResults,
        startIndex: 1,
        count: members.length,
        members,
        links: [],
      };
    },
    enabled: Boolean(groupId),
  });
}
