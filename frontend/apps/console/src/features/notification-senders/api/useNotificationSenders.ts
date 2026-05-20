/**
 * Copyright (c) 2025, WSO2 LLC. (https://www.wso2.com).
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

import {useThunderID} from '@thunderid/react';
import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import NotificationSenderQueryKeys from '../constants/query-keys';
import type {NotificationSenderListResponse} from '../models/notification-sender';

/**
 * Custom hook to fetch message notification senders from the server.
 *
 * @returns TanStack Query result object with notification senders data
 *
 * @example
 * ```tsx
 * function SendersList() {
 *   const { data, isLoading, error } = useNotificationSenders();
 *
 *   if (isLoading) return <div>Loading...</div>;
 *   if (error) return <div>Error: {error.message}</div>;
 *
 *   return (
 *     <ul>
 *       {data?.map((sender) => (
 *         <li key={sender.id}>{sender.name}</li>
 *       ))}
 *     </ul>
 *   );
 * }
 * ```
 */
export default function useNotificationSenders(): UseQueryResult<NotificationSenderListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<NotificationSenderListResponse>({
    queryKey: [NotificationSenderQueryKeys.NOTIFICATION_SENDERS, NotificationSenderQueryKeys.MESSAGE_SENDERS],
    queryFn: async (): Promise<NotificationSenderListResponse> => {
      const serverUrl: string = getServerUrl();

      const response: {
        data: NotificationSenderListResponse;
      } = await http.request({
        url: `${serverUrl}/notification-senders/message`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
