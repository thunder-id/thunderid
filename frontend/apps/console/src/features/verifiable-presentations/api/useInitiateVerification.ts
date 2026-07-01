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

import {useMutation, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import type {InitiateVerificationResponse} from '../models/vp';

/**
 * Initiates an OpenID4VP verification transaction for a presentation definition
 * (identified by its handle) and returns the wallet deep link to scan.
 */
export default function useInitiateVerification(): UseMutationResult<InitiateVerificationResponse, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useMutation<InitiateVerificationResponse, Error, string>({
    mutationFn: async (handle: string): Promise<InitiateVerificationResponse> => {
      const serverUrl: string = getServerUrl();
      const response: {data: InitiateVerificationResponse} = await http.request({
        url: `${serverUrl}/openid4vp/initiate`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data: {definition_id: handle, rp_id: 'console'},
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
  });
}
