// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import ConnectionQueryKeys from '../constants/query-keys';
import type {ConnectionResponse} from '../models/connection';
import {ConnectionTypes} from '../models/connection';
import type {CreateTrustedIssuerRequest, TrustedIssuer} from '../models/trusted-issuer';
import mapConnectionToTrustedIssuer from '../utils/mapConnectionToTrustedIssuer';

/**
 * Create a trusted issuer, i.e. a trust-only OIDC connection (POST /connections/oidc).
 *
 * Failures are not toasted here — the caller surfaces them inline next to the name field
 * (409 duplicate name) or next to the create action (any other failure).
 *
 * @example
 * ```tsx
 * const createTrustedIssuer = useCreateTrustedIssuer();
 * createTrustedIssuer.mutate({ouId, name, issuer, jwksEndpoint, idJagEnabled: true});
 * ```
 *
 * @public
 */
export default function useCreateTrustedIssuer(): UseMutationResult<TrustedIssuer, Error, CreateTrustedIssuerRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient: ReturnType<typeof useQueryClient> = useQueryClient();
  const {t} = useTranslation();
  const {showToast} = useToast();

  return useMutation<TrustedIssuer, Error, CreateTrustedIssuerRequest>({
    mutationFn: async (data: CreateTrustedIssuerRequest): Promise<TrustedIssuer> => {
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
  });
}
