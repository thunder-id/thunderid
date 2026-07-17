/**
 * Copyright (c) 2026, WSO2 LLC. (https://www.wso2.com).
 *
 * WSO2 LLC. licenses this file to you under the Apache License,
 * Version 2.0 (the "License"); you may not use this file except
 * in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an
 * "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
 * KIND, either express or implied. See the License for the
 * specific language governing permissions and limitations
 * under the License.
 */

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import SessionQueryKeys from '../constants/session-query-keys';
import type {SessionListFilter, SessionListResponse} from '../models/sessions';

/**
 * Custom hook to fetch the live sessions of a user or an application.
 *
 * @param filter - Exactly one of userId or appId
 * @param params - Optional pagination parameters
 * @returns TanStack Query result containing the paginated session list
 */
export default function useGetSessions(
  filter: SessionListFilter,
  params?: {limit?: number; offset?: number},
): UseQueryResult<SessionListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset} = params ?? {};
  const userId = 'userId' in filter ? filter.userId : undefined;
  const appId = 'appId' in filter ? filter.appId : undefined;

  return useQuery<SessionListResponse>({
    queryKey: [SessionQueryKeys.SESSIONS, {userId, appId, limit, offset}],
    queryFn: async (): Promise<SessionListResponse> => {
      const serverUrl: string = getServerUrl();
      const searchParams: URLSearchParams = new URLSearchParams();

      if (userId) {
        searchParams.append('userId', userId);
      }
      if (appId) {
        searchParams.append('appId', appId);
      }
      if (limit !== undefined) {
        searchParams.append('limit', String(limit));
      }
      if (offset !== undefined) {
        searchParams.append('offset', String(offset));
      }

      const response: {
        data: SessionListResponse;
      } = await http.request({
        url: `${serverUrl}/sessions?${searchParams.toString()}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(userId ?? appId),
    // Keep the previous page visible while the next one loads so server-side pagination does not
    // flicker to an empty grid or reset the row count between page changes.
    placeholderData: keepPreviousData,
  });
}
