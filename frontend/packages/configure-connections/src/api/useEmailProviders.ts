// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import {ConnectionInstanceCategories, type ConnectionInstance, type ConnectionListResponse} from '../models/connection';

/**
 * Custom hook to fetch email providers from the server.
 *
 * Backed by GET /connections?category=email-provider (server default page size — up to
 * 30 instances; larger deployments would need pagination support added here).
 *
 * @returns TanStack Query result object with email providers data
 *
 * @example
 * ```tsx
 * function ProvidersList() {
 *   const { data, isLoading, error } = useEmailProviders();
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.map((provider) => (
 *         <li key={provider.id}>{provider.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useEmailProviders(): UseQueryResult<ConnectionInstance[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionInstance[]>({
    queryKey: [ConnectionQueryKeys.CONNECTIONS, ConnectionQueryKeys.EMAIL_PROVIDERS],
    queryFn: async (): Promise<ConnectionInstance[]> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ConnectionListResponse;
      } = await http.request({
        url: `${serverUrl}/connections?category=${ConnectionInstanceCategories.EMAIL_PROVIDER}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data.connections;
    },
  });
}
