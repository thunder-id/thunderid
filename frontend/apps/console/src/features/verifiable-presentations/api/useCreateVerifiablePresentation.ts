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
import VerifiablePresentationQueryKeys from '../constants/vp-query-keys';
import type {CreateVerifiablePresentationRequest} from '../models/requests';
import type {VerifiablePresentation} from '../models/vp';

/**
 * Creates a new OpenID4VP presentation definition.
 */
export default function useCreateVerifiablePresentation(): UseMutationResult<
  VerifiablePresentation,
  Error,
  CreateVerifiablePresentationRequest
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation('verifiable-presentations');
  const {showToast} = useToast();

  return useMutation<VerifiablePresentation, Error, CreateVerifiablePresentationRequest>({
    mutationFn: async (data: CreateVerifiablePresentationRequest): Promise<VerifiablePresentation> => {
      const serverUrl: string = getServerUrl();
      const response: {data: VerifiablePresentation} = await http.request({
        url: `${serverUrl}/openid4vp/presentation-definitions`,
        method: 'POST',
        headers: {'Content-Type': 'application/json'},
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [VerifiablePresentationQueryKeys.VPS]}).catch(() => {
        /* noop */
      });
      showToast(t('create.success'), 'success');
    },
    onError: () => {
      showToast(t('create.error'), 'error');
    },
  });
}
