// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {ApplyResult, RevertGatewayRequest} from '../models/gateway';

export default function useRevertGateway(): UseMutationResult<
  ApplyResult,
  Error,
  {gatewayId: string; data: RevertGatewayRequest}
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<ApplyResult, Error, {gatewayId: string; data: RevertGatewayRequest}>({
    mutationFn: async ({gatewayId, data}): Promise<ApplyResult> => {
      const response: {data: ApplyResult} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/revert`,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (result, {gatewayId}) => {
      if (!result.recorded) return;
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.APPLIED_VERSION, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_DIFF, gatewayId]});
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, gatewayId]});
    },
  });
}
