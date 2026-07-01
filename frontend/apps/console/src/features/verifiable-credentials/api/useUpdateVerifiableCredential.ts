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

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import VerifiableCredentialQueryKeys from '../constants/vc-query-keys';
import type {UpdateVerifiableCredentialRequest} from '../models/requests';
import type {VerifiableCredential} from '../models/vc';

interface UpdateArgs {
  id: string;
  data: UpdateVerifiableCredentialRequest;
}

/**
 * Updates an existing OpenID4VCI credential configuration.
 */
export default function useUpdateVerifiableCredential(): UseMutationResult<VerifiableCredential, Error, UpdateArgs> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('verifiable-credentials');
  const {showToast} = useToast();

  return useMutation<VerifiableCredential, Error, UpdateArgs>({
    mutationFn: async ({id, data}: UpdateArgs): Promise<VerifiableCredential> => {
      const serverUrl: string = getServerUrl();
      const response: {data: VerifiableCredential} = await http.request({
        url: `${serverUrl}/openid4vci/credential-configurations/${id}`,
        method: 'PUT',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result, {id}: UpdateArgs) => {
      queryClient.invalidateQueries({queryKey: [VerifiableCredentialQueryKeys.VCS]}).catch(() => {
        /* noop */
      });
      queryClient.invalidateQueries({queryKey: [VerifiableCredentialQueryKeys.VC, id]}).catch(() => {
        /* noop */
      });
      showToast(t('update.success'), 'success');
    },
    onError: () => {
      showToast(t('update.error'), 'error');
    },
  });
}
