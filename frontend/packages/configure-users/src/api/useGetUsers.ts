// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {ApiFilteringParams} from '@thunderid/types';
import UserQueryKeys from '../constants/user-query-keys';
import type {UserListResponse} from '../models/users';

/**
 * Custom hook to fetch a list of users. With a `projectId`, only that project's users are listed;
 * without one, every user is.
 *
 * @param params - Optional query parameters for filtering, pagination and project
 * @returns TanStack Query result object containing user list data, loading state, and error information
 */
export default function useGetUsers(
  params?: ApiFilteringParams & {projectId?: string},
): UseQueryResult<UserListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset, filter, projectId} = params ?? {};

  return useQuery<UserListResponse>({
    queryKey: [UserQueryKeys.USERS, {limit, offset, filter, projectId}],
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
      if (projectId) {
        searchParams.append('projectId', projectId);
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
