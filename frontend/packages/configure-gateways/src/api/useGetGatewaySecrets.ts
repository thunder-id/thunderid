// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {GatewaySecretList, GatewayValueListParams} from '../models/gateway';

/**
 * Lists the secrets a gateway holds. The request passes through this plane to the gateway's own
 * store, so a gateway that cannot be reached fails it with a 502.
 */
export default function useGetGatewaySecrets(
  gatewayId: string,
  params: GatewayValueListParams = {},
): UseQueryResult<GatewaySecretList> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit, offset} = params;

  return useQuery<GatewaySecretList>({
    queryKey: [GatewayQueryKeys.GATEWAY_SECRETS, gatewayId, {limit, offset}],
    queryFn: async (): Promise<GatewaySecretList> => {
      const query = new URLSearchParams();
      if (limit !== undefined) query.set('limit', String(limit));
      if (offset !== undefined) query.set('offset', String(offset));
      const suffix = query.toString() ? `?${query.toString()}` : '';
      const response: {data: GatewaySecretList} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/secrets${suffix}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(gatewayId),
    placeholderData: keepPreviousData,
  });
}
