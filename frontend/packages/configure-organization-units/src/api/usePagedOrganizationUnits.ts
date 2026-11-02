// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useInfiniteQuery, type UseInfiniteQueryResult, type InfiniteData} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import fetchChildOrganizationUnits from './fetchChildOrganizationUnits';
import fetchOrganizationUnits from './fetchOrganizationUnits';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import type {OrganizationUnitListResponse} from '../models/responses';

const PAGE_SIZE = 50;

/**
 * Loads organization units page by page: the root units of every tree when `parentId` is `null`,
 * otherwise the direct children of `parentId`. Pickers call `fetchNextPage` for "Load more".
 *
 * @param parentId - The unit whose children to load, or `null` for the root units
 * @param enabled - Whether to load
 * @returns TanStack infinite query result of organization unit pages
 *
 * @public
 */
export default function usePagedOrganizationUnits(
  parentId: string | null,
  enabled = true,
): UseInfiniteQueryResult<InfiniteData<OrganizationUnitListResponse>> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useInfiniteQuery({
    queryKey: [
      parentId ? OrganizationUnitQueryKeys.CHILD_ORGANIZATION_UNITS : OrganizationUnitQueryKeys.ORGANIZATION_UNITS,
      'paged',
      parentId,
    ],
    queryFn: ({pageParam}): Promise<OrganizationUnitListResponse> => {
      const params = {limit: PAGE_SIZE, offset: pageParam};
      return parentId
        ? fetchChildOrganizationUnits(http, getServerUrl(), parentId, params)
        : fetchOrganizationUnits(http, getServerUrl(), params);
    },
    initialPageParam: 0,
    getNextPageParam: (lastPage, pages) => {
      const loaded = pages.reduce((count, page) => count + page.organizationUnits.length, 0);
      return lastPage.organizationUnits.length > 0 && loaded < lastPage.totalResults ? loaded : undefined;
    },
    enabled,
  });
}
