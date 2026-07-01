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

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import VerifiablePresentationQueryKeys from '../constants/vp-query-keys';
import type {VerifiablePresentation} from '../models/vp';

/**
 * Fetches a single OpenID4VP presentation definition by id.
 */
export default function useGetVerifiablePresentation(id: string): UseQueryResult<VerifiablePresentation> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<VerifiablePresentation>({
    queryKey: [VerifiablePresentationQueryKeys.VP, id],
    enabled: Boolean(id),
    queryFn: async (): Promise<VerifiablePresentation> => {
      const serverUrl: string = getServerUrl();
      const response: {data: VerifiablePresentation} = await http.request({
        url: `${serverUrl}/openid4vp/presentation-definitions/${id}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
