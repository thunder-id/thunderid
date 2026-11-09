// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import fetchSharingPolicies from './fetchSharingPolicies';
import ResourceSharingQueryKeys from '../constants/resource-sharing-query-keys';
import type {MandatableRole, MandatableRoles} from '../models/mandatable-role';
import policyReachesChildUnit from '../utils/policyReachesChildUnit';

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
 * The issuer may not list the child unit's roles, so the hook lists the issuer's own roles and keeps
 * those whose sharing policy, as the issuer states it, reaches the child unit. A role is mandated only
 * in a child unit the user share names, which is a direct child of the issuer, and only the issuer's
 * own policy reaches a direct child.
 * Called directly rather than through `configure-roles`, which depends on this package.
 * @param ouId - The child unit the roles would be mandated in
 * @param issuerOuId - The unit issuing the share
 * @param enabled - Whether to fetch
 * @returns TanStack Query result holding the candidates, in the order the server lists them, and the
 * names of every role the issuer can see
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

      const ofIssuer: ListedRole[] = [];
      for (;;) {
        const params = new URLSearchParams({
          filter: `ouId eq "${issuerOuId}"`,
          limit: PAGE_SIZE.toString(),
          offset: ofIssuer.length.toString(),
        });
        const response: {data: RoleListPage} = await http.request({
          url: `${serverUrl}/roles?${params.toString()}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        const page = response.data.roles ?? [];
        ofIssuer.push(...page);

        if (page.length === 0 || ofIssuer.length >= response.data.totalResults) break;
      }

      const owned = ofIssuer.filter((role) => role.isOwnedByOU === true);
      const reaches = await Promise.all(
        owned.map(async (role) => {
          const policies = await fetchSharingPolicies(http, serverUrl, `roles/${role.id}`);
          return policies.some(
            (policy) => policy.initiatingOuId === issuerOuId && policyReachesChildUnit(policy, ouId),
          );
        }),
      );

      const candidates: MandatableRole[] = owned
        .filter((_, index) => reaches[index])
        .map(({id, name, description}) => ({id, name, description, ouId: issuerOuId}));
      const names: Record<string, string> = Object.fromEntries(ofIssuer.map((role) => [role.id, role.name]));

      return {candidates, names};
    },
    enabled: enabled && Boolean(ouId) && Boolean(issuerOuId),
  });
}
