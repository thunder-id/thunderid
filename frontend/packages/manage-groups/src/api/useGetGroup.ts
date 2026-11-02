// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {getGroup, GroupQueryKeys, useManagementRequestConfig, type Group} from '@thunderid/react';

/**
 * Fetches a group, for the heading of its details page.
 *
 * @param groupId - The ID of the group to fetch.
 * @returns TanStack Query result holding the group.
 */
export default function useGetGroup(groupId: string): UseQueryResult<Group> {
  const config = useManagementRequestConfig('groups');

  return useQuery<Group>({
    enabled: Boolean(groupId),
    queryFn: (): Promise<Group> => getGroup({...config, groupId}),
    queryKey: [GroupQueryKeys.GROUP, groupId],
  });
}
