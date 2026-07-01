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
import VerifiableCredentialQueryKeys from '../constants/vc-query-keys';
import type {VCListResponse} from '../models/vc';

/**
 * Fetches all OpenID4VCI credential configurations.
 */
export default function useGetVerifiableCredentials(): UseQueryResult<VCListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<VCListResponse>({
    queryKey: [VerifiableCredentialQueryKeys.VCS],
    queryFn: async (): Promise<VCListResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {data: VCListResponse} = await http.request({
        url: `${serverUrl}/openid4vci/credential-configurations`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data ?? [];
    },
  });
}
