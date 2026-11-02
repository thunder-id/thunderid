// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {AUTHZEN_PDP_CONNECTIONS_PAGE_SIZE} from '../constants/authzen-pdp-constants';
import ResourceServerQueryKeys from '../constants/resource-server-query-keys';
import type {AuthZENPDPConnectionListResponse, AuthZENPDPConnectionSummary} from '../models/resource-server';

export default function useAuthZENPDPConnections(): UseQueryResult<AuthZENPDPConnectionSummary[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AuthZENPDPConnectionSummary[]>({
    queryKey: [ResourceServerQueryKeys.AUTHZEN_PDP_CONNECTIONS],
    queryFn: async (): Promise<AuthZENPDPConnectionSummary[]> => {
      const serverUrl = getServerUrl();
      const fetchPage = async (offset: number): Promise<AuthZENPDPConnectionListResponse> => {
        const response: {data: AuthZENPDPConnectionListResponse} = await http.request({
          url: `${serverUrl}/connections?category=authorization-pdp&limit=${AUTHZEN_PDP_CONNECTIONS_PAGE_SIZE}&offset=${offset}`,
          method: 'GET',
        } as unknown as Parameters<typeof http.request>[0]);

        return response.data;
      };

      const firstPage = await fetchPage(0);
      const remainingOffsets = Array.from(
        {
          length: Math.max(0, Math.ceil(firstPage.totalResults / AUTHZEN_PDP_CONNECTIONS_PAGE_SIZE) - 1),
        },
        (_, index) => (index + 1) * AUTHZEN_PDP_CONNECTIONS_PAGE_SIZE,
      );
      const remainingPages = await Promise.all(remainingOffsets.map((offset) => fetchPage(offset)));

      return [firstPage, ...remainingPages].flatMap((page) => page.connections);
    },
  });
}
