// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import {ConnectionInstanceCategories, type ConnectionListResponse} from '../models/connection';
import {IdentityProviderTypes, type IdentityProviderType} from '../models/identity-provider';
import type {IdentityProviderListResponse} from '../models/responses';

/**
 * Maps the lowercase vendor `type` served by GET /connections (e.g. "google") to the
 * UPPERCASE IdentityProviderType this hook has always returned (e.g. "GOOGLE"), so consumers
 * (executor type maps, icon/label lookups, and their tests) don't need to change.
 */
const VENDOR_TO_IDP_TYPE: Record<string, IdentityProviderType> = {
  google: IdentityProviderTypes.GOOGLE,
  github: IdentityProviderTypes.GITHUB,
  oidc: IdentityProviderTypes.OIDC,
  oauth: IdentityProviderTypes.OAUTH,
  esignet: IdentityProviderTypes.ESIGNET,
};

/**
 * Custom hook to fetch identity providers from the server.
 *
 * Backed by GET /connections?category=identity-provider (server default page size — up to
 * 30 instances; larger deployments would need pagination support added here).
 *
 * @returns TanStack Query result object with identity providers data
 *
 * @example
 * ```tsx
 * function IntegrationsList() {
 *   const { data, isLoading, error } = useIdentityProviders();
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.map((idp) => (
 *         <li key={idp.id}>{idp.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useIdentityProviders(): UseQueryResult<IdentityProviderListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<IdentityProviderListResponse>({
    queryKey: [ConnectionQueryKeys.CONNECTIONS, ConnectionQueryKeys.IDENTITY_PROVIDERS],
    queryFn: async (): Promise<IdentityProviderListResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ConnectionListResponse;
      } = await http.request({
        url: `${serverUrl}/connections?category=${ConnectionInstanceCategories.IDENTITY_PROVIDER}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.connections
        .filter((connection) => VENDOR_TO_IDP_TYPE[connection.type] !== undefined)
        .map((connection) => ({
          id: connection.id,
          name: connection.name,
          description: connection.description,
          type: VENDOR_TO_IDP_TYPE[connection.type],
        }));
    },
  });
}
