// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQueries} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useMemo} from 'react';
import OrganizationUnitQueryKeys from '../constants/organization-unit-query-keys';
import type {OrganizationUnit} from '../models/organization-unit';

/**
 * Fetches several organization units by id, sharing the cache {@link useGetOrganizationUnit} uses.
 *
 * Sharing policies name units only by id, so this is how their names are shown. A unit that fails
 * to load is simply absent from the map, and callers fall back to showing its id.
 *
 * @param ids - The organization unit ids to fetch
 * @returns A map of id to organization unit, for the units loaded so far
 *
 * @public
 */
export default function useGetOrganizationUnitsByIds(ids: string[]): Map<string, OrganizationUnit> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const uniqueIds = useMemo(() => [...new Set(ids)].sort(), [ids]);

  return useQueries({
    queries: uniqueIds.map((id) => ({
      queryKey: [OrganizationUnitQueryKeys.ORGANIZATION_UNIT, id],
      queryFn: async (): Promise<OrganizationUnit> => {
        const response: {data: OrganizationUnit} = await http.request({
          url: `${getServerUrl()}/organization-units/${encodeURIComponent(id)}`,
          method: 'GET',
          headers: {'Content-Type': 'application/json'},
        } as unknown as Parameters<typeof http.request>[0]);
        return response.data;
      },
      staleTime: 60_000,
    })),
    combine: (results) => {
      const map = new Map<string, OrganizationUnit>();
      results.forEach((result) => {
        if (result.data) map.set(result.data.id, result.data);
      });
      return map;
    },
  });
}
