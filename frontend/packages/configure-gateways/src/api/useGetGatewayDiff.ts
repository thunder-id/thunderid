// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {GatewayDiff} from '../models/gateway';

/**
 * Compares a version (a number or `latest`) with what the gateway holds. Nothing is sent to the
 * gateway. Pass `enabled: false` to hold the request until it is needed.
 */
export default function useGetGatewayDiff(
  gatewayId: string,
  version: string,
  enabled = true,
): UseQueryResult<GatewayDiff> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<GatewayDiff>({
    queryKey: [GatewayQueryKeys.GATEWAY_DIFF, gatewayId, version],
    queryFn: async (): Promise<GatewayDiff> => {
      const query = new URLSearchParams({version});
      const response: {data: GatewayDiff} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/diff?${query.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: enabled && Boolean(gatewayId) && Boolean(version),
  });
}
