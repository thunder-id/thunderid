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
import TrustedIssuerQueryKeys from '../constants/query-keys';
import {ConnectionQueryKeys, ConnectionTypes} from '@thunderid/configure-connections';

/**
 * Delete a trusted issuer (DELETE /connections/oidc/{id}).
 *
 * @example
 * ```tsx
 * const deleteTrustedIssuer = useDeleteTrustedIssuer();
 * deleteTrustedIssuer.mutate(id);
 * ```
 *
 * @public
 */
export default function useDeleteTrustedIssuer(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      const serverUrl: string = getServerUrl();
      await http.request({
        url: `${serverUrl}/connections/${ConnectionTypes.OIDC}/${id}`,
        method: 'DELETE',
        headers: {
          'Content-Type': 'application/json',
        },
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_data, id) => {
      queryClient.removeQueries({queryKey: [TrustedIssuerQueryKeys.TRUSTED_ISSUER, id]});
      queryClient.invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTIONS]}).catch(() => {
        // Ignore invalidation errors
      });
      queryClient
        .invalidateQueries({queryKey: [ConnectionQueryKeys.CONNECTION_INSTANCES, ConnectionTypes.OIDC]})
        .catch(() => {
          // Ignore invalidation errors
        });
      showToast(t('trustedIssuers:delete.success', 'Trusted issuer deleted successfully.'), 'success');
    },
    onError: () => {
      showToast(t('trustedIssuers:delete.error', 'Failed to delete trusted issuer. Please try again.'), 'error');
    },
  });
}
