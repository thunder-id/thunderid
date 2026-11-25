// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery} from '@tanstack/react-query';
import {useConfig, type Capabilities} from '@thunderid/contexts';
import {useGrantedPermissions, useThunderID, useUser} from '@thunderid/react';
import {useCallback} from 'react';

export type {Capabilities};

/**
 * The body of `GET /capabilities`: the management actions the signed-in admin's token allows.
 */
export interface CapabilitiesResponse {
  scope: {type: 'deployment' | 'ou'; ouId?: string};
  /** Each catalog resource (`app`, `user`, ...) mapped to its actions (`view`, `create`, ...). */
  resources: Record<string, Record<string, boolean>>;
}

/**
 * Reads what the signed-in admin may do, to decide which pages and actions the Console offers.
 */
export default function useCapabilities(): Capabilities {
  const {http, isSignedIn} = useThunderID();
  const {getServerUrl} = useConfig();
  const {permissions, isLoading: isLoadingPermissions} = useGrantedPermissions();
  const {flattenedProfile} = useUser();
  const ouId: unknown = (flattenedProfile as {ouId?: unknown} | null)?.ouId;

  // Keyed by what the token carries, so a token with other scopes or another organization unit reads
  // them again. The last answer stays while the new one loads, so a guarded page is not unmounted mid-edit.
  const {data, isPending, error, refetch} = useQuery<CapabilitiesResponse>({
    queryKey: ['capabilities', [...permissions].sort(), typeof ouId === 'string' ? ouId : ''],
    enabled: isSignedIn && !isLoadingPermissions,
    queryFn: async (): Promise<CapabilitiesResponse> => {
      const response: {data: CapabilitiesResponse} = await http.request({
        url: `${getServerUrl()}/capabilities`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    placeholderData: keepPreviousData,
    staleTime: Infinity,
    retry: 1,
  });

  // Nothing is allowed until the capabilities are read, so an admin is offered only what the server reports.
  const can = useCallback(
    (resource: string, action = 'view'): boolean => Boolean(data?.resources[resource]?.[action]),
    [data],
  );

  const retry = useCallback((): void => {
    refetch().catch(() => undefined);
  }, [refetch]);

  return {can, isLoading: isPending, error, retry};
}
