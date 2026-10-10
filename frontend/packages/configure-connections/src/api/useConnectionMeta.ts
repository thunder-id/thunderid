// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionMetaResponse, ConnectionType, ConnectionMetaVendor} from '../models/connection';

/**
 * Fetches what a connection vendor can be configured with.
 *
 * Backed by GET /connections/meta?vendor={vendor}, which answers with a list holding that vendor
 * alone; the hook unwraps it to the vendor's entry. Only authentication is described today: the
 * methods the vendor supports and the fields each takes. Rendering from this rather than from a
 * hardcoded list is what lets a method the console was not built against still appear.
 *
 * @param vendor - The connection type to describe. Pass undefined to skip the request.
 * @returns TanStack Query result carrying the vendor's metadata, or undefined if the server
 * does not describe it
 *
 * @example
 * ```tsx
 * const {data} = useConnectionMeta('email-smtp');
 * const methods = data?.authentication.methods ?? [];
 * ```
 */
export default function useConnectionMeta(vendor?: ConnectionType): UseQueryResult<ConnectionMetaVendor | undefined> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionMetaResponse, Error, ConnectionMetaVendor | undefined>({
    enabled: Boolean(vendor),
    queryKey: [ConnectionQueryKeys.CONNECTIONS, ConnectionQueryKeys.CONNECTION_META, vendor],
    queryFn: async (): Promise<ConnectionMetaResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: ConnectionMetaResponse;
      } = await http.request({
        url: `${serverUrl}/connections/meta?vendor=${vendor}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    select: (data: ConnectionMetaResponse): ConnectionMetaVendor | undefined =>
      data.vendors.find((entry: ConnectionMetaVendor) => entry.vendor === vendor),
  });
}
