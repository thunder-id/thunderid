// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {ApplyResult, RevertGatewayRequest} from '../models/gateway';

/**
 * Runs a dry run of reverting a gateway, which reports the variables and secrets the version
 * reverted to refers to that the gateway does not hold. Nothing on the gateway changes. Pass
 * `enabled: false` to hold the request until it is needed.
 */
export default function useRevertDryRun(gatewayId: string, enabled = true): UseQueryResult<ApplyResult> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<ApplyResult>({
    queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, gatewayId, 'revert'],
    queryFn: async (): Promise<ApplyResult> => {
      const data: RevertGatewayRequest = {dryRun: true};
      const response: {data: ApplyResult} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/revert`,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: enabled && Boolean(gatewayId),
    refetchOnWindowFocus: false,
  });
}
