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
import type {TrustedIssuer, TrustedIssuerFormData} from '../models/trusted-issuer';
import mapConnectionToTrustedIssuer from '../utils/mapConnectionToTrustedIssuer';
import {
  ConnectionQueryKeys,
  ConnectionTypes,
  isConflictError,
  type ConnectionResponse,
} from '@thunderid/configure-connections';

/**
 * Create a trusted issuer, i.e. a trust-only OIDC connection (POST /connections/oidc).
 *
 * Conflicts (409 duplicate name) are not toasted here — the caller surfaces them inline next
 * to the name field.
 *
 * @example
 * ```tsx
 * const createTrustedIssuer = useCreateTrustedIssuer();
 * createTrustedIssuer.mutate({name, issuer, jwksEndpoint, idJagEnabled: true});
 * ```
 *
 * @public
 */
export default function useCreateTrustedIssuer(): UseMutationResult<TrustedIssuer, Error, TrustedIssuerFormData> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<TrustedIssuer, Error, TrustedIssuerFormData>({
    mutationFn: async (data: TrustedIssuerFormData): Promise<TrustedIssuer> => {
      const serverUrl: string = getServerUrl();
      const response: {
        data: ConnectionResponse;
      } = await http.request({
        url: `${serverUrl}/connections/${ConnectionTypes.OIDC}`,
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return mapConnectionToTrustedIssuer(response.data);
    },
    onSuccess: () => {
      queryClient.invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient
        .invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTION_INSTANCES, ConnectionTypes.OIDC]})
        .catch(() => {
          // Ignore invalidation errors
        });
      showToast(t('trustedIssuers:create.success', 'Trusted issuer created successfully.'), 'success');
    },
    onError: (error) => {
      if (!isConflictError(error)) {
        showToast(t('trustedIssuers:create.error', 'Failed to create trusted issuer. Please try again.'), 'error');
      }
    },
  });
}
