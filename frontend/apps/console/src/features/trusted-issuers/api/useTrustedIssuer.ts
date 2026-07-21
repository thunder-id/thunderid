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
import TrustedIssuerQueryKeys from '../constants/query-keys';
import type {TrustedIssuer} from '../models/trusted-issuer';
import mapConnectionToTrustedIssuer from '../utils/mapConnectionToTrustedIssuer';
import {ConnectionTypes, type ConnectionResponse} from '@thunderid/configure-connections';

/**
 * Fetch a single trusted issuer (GET /connections/oidc/{id}). Disabled until an id is provided.
 *
 * @example
 * ```tsx
 * const {data: trustedIssuer, isLoading} = useTrustedIssuer(id);
 * ```
 *
 * @public
 */
export default function useTrustedIssuer(id: string | undefined): UseQueryResult<TrustedIssuer> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<TrustedIssuer>({
    queryKey: [TrustedIssuerQueryKeys.TRUSTED_ISSUER, id],
    enabled: Boolean(id),
    queryFn: async (): Promise<TrustedIssuer> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionResponse;
      } = await http.request({
        url: `${serverUrl}/connections/${ConnectionTypes.OIDC}/${id}`,
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);

      return mapConnectionToTrustedIssuer(response.data);
    },
  });
}
