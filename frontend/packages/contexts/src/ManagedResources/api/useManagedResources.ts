// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useThunderID} from '@thunderid/react';
import useConfig from '../../Config/useConfig';

/**
 * The resource types the server reports ownership for.
 */
export type ManagedResourceType =
  | 'application'
  | 'user'
  | 'group'
  | 'role'
  | 'connection'
  | 'flow'
  | 'theme'
  | 'layout'
  | 'user_type'
  | 'organization_unit'
  | 'resource_server'
  | 'agent'
  | 'translation'
  | 'presentation_definition'
  | 'credential_configuration'
  | 'server_config';

export interface ManagedResourcesResponse {
  /**
   * Whether this deployment tracks control-plane ownership at all. False on a server that has no
   * control plane in front of it, in which case nothing is read only.
   */
  enabled: boolean;
  /**
   * The ids owned by the control plane, keyed by resource type.
   */
  managed: Partial<Record<ManagedResourceType, string[]>>;
}

/**
 * Reads which resources this deployment received from a control plane.
 *
 * Those resources can only be changed where they came from: an edit made here would survive only
 * until the next promotion overwrote it, and the server refuses it with 403. The console asks once
 * and reuses the answer, rather than every resource carrying a flag of its own.
 */
export default function useManagedResources(): UseQueryResult<ManagedResourcesResponse, Error> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ManagedResourcesResponse, Error>({
    queryKey: ['managed-resources'],
    queryFn: async (): Promise<ManagedResourcesResponse> => {
      const serverUrl: string = getServerUrl();
      const response = await http.request({
        url: `${serverUrl}/managed-resources`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return (response as {data: ManagedResourcesResponse}).data;
    },
    // Ownership only changes when the control plane applies, so it does not need refetching per view.
    staleTime: 5 * 60 * 1000,
    // A server that does not expose the endpoint is simply not control-plane managed.
    retry: false,
  });
}
