// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import UserTypeQueryKeys from '../constants/userTypeQueryKeys';
import type {UserTypeUsagesResponse} from '../types/user-types';

/**
 * Fetch the resources that reference a user type, such as users created with it
 * (GET /user-types/{id}/usages). Used to populate the pre-delete confirmation dialog.
 *
 * @param userTypeId - The unique identifier of the user type
 * @param enabled - Whether the query should run (default true)
 * @returns TanStack Query result with user type usages data
 */
export default function useGetUserTypeUsages(
  userTypeId: string | null,
  enabled = true,
): UseQueryResult<UserTypeUsagesResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<UserTypeUsagesResponse>({
    queryKey: [UserTypeQueryKeys.USER_TYPE_USAGES, userTypeId],
    queryFn: async (): Promise<UserTypeUsagesResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {data: UserTypeUsagesResponse} = await http.request({
        url: `${serverUrl}/user-types/${encodeURIComponent(userTypeId!)}/usages`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(userTypeId) && enabled,
  });
}
