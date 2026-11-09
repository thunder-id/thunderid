// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import UserQueryKeys from '../constants/user-query-keys';
import type {OwnedAgentListResponse} from '../models/users';

/**
 * The most agents the list returns in one page.
 */
const MAX_OWNED_AGENTS = 100;

/**
 * Custom hook to fetch the agents that name a user as their owner.
 *
 * @param userId - The unique identifier of the user
 * @returns TanStack Query result with the user's agents
 */
export default function useGetUserOwnedAgents(userId: string): UseQueryResult<OwnedAgentListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<OwnedAgentListResponse>({
    queryKey: [UserQueryKeys.USER_OWNED_AGENTS, userId],
    queryFn: async (): Promise<OwnedAgentListResponse> => {
      const serverUrl: string = getServerUrl();
      const queryParams = new URLSearchParams({
        filter: `owner eq "${userId}"`,
        limit: String(MAX_OWNED_AGENTS),
        include: 'display',
      });

      const response: {data: OwnedAgentListResponse} = await http.request({
        url: `${serverUrl}/agents?${queryParams.toString()}`,
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
