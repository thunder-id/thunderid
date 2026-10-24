// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';

/**
 * The part of an organization unit the connections screens show.
 */
export interface ConnectionOU {
  id: string;
  handle: string;
  name: string;
}

/**
 * Fetch the organization unit a connection belongs to (GET /organization-units/{id}), for its handle. Disabled until
 * an id is provided, so a connection with no owner makes no request.
 */
export default function useConnectionOU(ouId: string | undefined): UseQueryResult<ConnectionOU> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ConnectionOU>({
    queryKey: [ConnectionQueryKeys.CONNECTION_OU, ouId],
    enabled: Boolean(ouId),
    queryFn: async (): Promise<ConnectionOU> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionOU;
      } = await http.request({
        url: `${serverUrl}/organization-units/${encodeURIComponent(ouId!)}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
