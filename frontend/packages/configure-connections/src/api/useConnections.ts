// Copyright 2025 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionInstanceCategory, ConnectionListResponse} from '../models/connection';

export interface UseConnectionsParams {
  category?: ConnectionInstanceCategory;
  /**
   * Filter expression, for example `ouId eq "<id>"`, which lists what that organization unit owns and was shared.
   */
  filter?: string;
  limit?: number;
  offset?: number;
}

export interface UseConnectionsOptions {
  enabled?: boolean;
}

/**
 * Fetch a paginated list of the configured connection instances (GET /connections),
 * optionally filtered by functional category. Returns the full paginated envelope.
 */
export default function useConnections(
  params?: UseConnectionsParams,
  options?: UseConnectionsOptions,
): UseQueryResult<ConnectionListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {category, filter, limit, offset} = params ?? {};

  return useQuery<ConnectionListResponse>({
    queryKey: [ConnectionQueryKeys.CONNECTIONS, {category, filter, limit, offset}],
    enabled: options?.enabled ?? true,
    queryFn: async (): Promise<ConnectionListResponse> => {
      const serverUrl: string = getServerUrl();
      const searchParams: URLSearchParams = new URLSearchParams();
      if (category) {
        searchParams.set('category', category);
      }
      if (filter) {
        searchParams.set('filter', filter);
      }
      if (limit !== undefined) {
        searchParams.set('limit', String(limit));
      }
      if (offset !== undefined) {
        searchParams.set('offset', String(offset));
      }
      const queryString: string = searchParams.toString();
      const query: string = queryString ? `?${queryString}` : '';
      const response: {
        data: ConnectionListResponse;
      } = await http.request({
        url: `${serverUrl}/connections${query}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
