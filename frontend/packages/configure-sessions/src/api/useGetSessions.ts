// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SessionQueryKeys from '../constants/session-query-keys';
import type {SessionFilter, SessionListResponse} from '../models/session';

/**
 * Custom hook to fetch one page of the live sessions of a user or of an application.
 *
 * @param filter - The user or application whose sessions to list
 * @param limit - Page size
 * @param offset - Number of sessions to skip
 * @returns TanStack Query result object containing the page of sessions
 */
export default function useGetSessions(
  filter: SessionFilter,
  limit: number,
  offset: number,
): UseQueryResult<SessionListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const params = new URLSearchParams({...filter, limit: String(limit), offset: String(offset)});

  return useQuery<SessionListResponse>({
    queryKey: [SessionQueryKeys.SESSIONS, filter, {limit, offset}],
    queryFn: async (): Promise<SessionListResponse> => {
      const response: {
        data: SessionListResponse;
      } = await http.request({
        url: `${getServerUrl()}/sessions?${params.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    placeholderData: keepPreviousData,
  });
}
