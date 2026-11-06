// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ResourceSharingQueryKeys from '../constants/resource-sharing-query-keys';
import type {MandatableRole, MandatableRoles} from '../models/mandatable-role';

interface ListedRole {
  id: string;
  name: string;
  description?: string;
  isOwnedByOU?: boolean;
}

interface RoleListPage {
  totalResults: number;
  roles?: ListedRole[];
}

const PAGE_SIZE = 100;

/**
 * Fetches the roles an issuing unit can mandate in one of its child units: the issuer's own roles that
 * are shared with the child unit. Roles the child unit owns itself are left out, because the issuer
 * has no say over them.
 *
 * A role listed for a unit it is shared with carries that unit's ID, not its owner's, so ownership is
 * read from the issuer's own listing.
 *
 * Called directly rather than through `configure-roles`, which depends on this package.
 *
 * @param ouId - The child unit the roles would be mandated in
 * @param issuerOuId - The unit issuing the share
 * @param enabled - Whether to fetch
 * @returns TanStack Query result holding the candidates, in the order the server lists them, and the
 * names of every role the unit can see
 *
 * @public
 */
export default function useGetMandatableRoles(
  ouId: string,
  issuerOuId: string,
  enabled = true,
): UseQueryResult<MandatableRoles> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<MandatableRoles>({
    queryKey: [ResourceSharingQueryKeys.MANDATABLE_ROLES, ouId, issuerOuId],
    queryFn: async (): Promise<MandatableRoles> => {
      const serverUrl: string = getServerUrl();

      const listRoles = async (listedOuId: string): Promise<ListedRole[]> => {
        const listed: ListedRole[] = [];
        for (;;) {
          const params = new URLSearchParams({
            filter: `ouId eq "${listedOuId}"`,
            limit: PAGE_SIZE.toString(),
            offset: listed.length.toString(),
          });
          const response: {data: RoleListPage} = await http.request({
            url: `${serverUrl}/roles?${params.toString()}`,
            method: 'GET',
          } as unknown as Parameters<typeof http.request>[0]);

          const page = response.data.roles ?? [];
          listed.push(...page);

          if (page.length === 0 || listed.length >= response.data.totalResults) break;
        }
        return listed;
      };

      const [inUnit, ofIssuer] = await Promise.all([listRoles(ouId), listRoles(issuerOuId)]);
      const ownedByIssuer = new Set(ofIssuer.filter((role) => role.isOwnedByOU === true).map((role) => role.id));

      const candidates: MandatableRole[] = inUnit
        .filter((role) => role.isOwnedByOU === false && ownedByIssuer.has(role.id))
        .map(({id, name, description}) => ({id, name, description, ouId: issuerOuId}));
      const names: Record<string, string> = Object.fromEntries(inUnit.map((role) => [role.id, role.name]));

      return {candidates, names};
    },
    enabled: enabled && Boolean(ouId) && Boolean(issuerOuId),
  });
}
