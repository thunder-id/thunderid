// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery} from '@tanstack/react-query';
import {getOrganizationUnit, useManagementRequestConfig, useUser, type OrganizationUnit} from '@thunderid/react';

/**
 * The organization unit the signed-in admin manages, by the friendliest label available: its name,
 * then its handle. Read from the token when it carries them, otherwise looked up by the token's
 * organization unit ID.
 *
 * @returns The label, or `undefined` while it loads or when none is known.
 */
export default function useOrganizationName(): string | undefined {
  const {flattenedProfile} = useUser();
  const config = useManagementRequestConfig('organizationUnits');
  const profile = (flattenedProfile ?? {}) as {ouHandle?: unknown; ouId?: unknown; ouName?: unknown};
  const fromToken: string | undefined = [profile.ouName, profile.ouHandle].find(
    (value: unknown): value is string => typeof value === 'string' && value.length > 0,
  );
  const ouId: string = typeof profile.ouId === 'string' ? profile.ouId : '';

  const {data} = useQuery<OrganizationUnit>({
    enabled: !fromToken && Boolean(ouId),
    queryFn: (): Promise<OrganizationUnit> => getOrganizationUnit({...config, ouId}),
    queryKey: ['organization-unit', ouId],
    staleTime: Infinity,
  });

  return fromToken ?? data?.name ?? data?.handle;
}
