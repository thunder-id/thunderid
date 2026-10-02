// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {GatewayVariable, UpdateGatewayValueRequest} from '../models/gateway';

export default function useUpdateGatewayVariable(): UseMutationResult<
  GatewayVariable,
  Error,
  {gatewayId: string; name: string; data: UpdateGatewayValueRequest}
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {showToast} = useToast();
  const {t} = useTranslation();
  const queryClient = useQueryClient();

  return useMutation<GatewayVariable, Error, {gatewayId: string; name: string; data: UpdateGatewayValueRequest}>({
    mutationFn: async ({gatewayId, name, data}): Promise<GatewayVariable> => {
      const response: {data: GatewayVariable} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/variables/${encodeURIComponent(name)}`,
        method: 'PUT',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result, {gatewayId, name}) => {
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_VARIABLES, gatewayId]});
      // What the gateway holds decides what an apply or a revert would find missing.
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, gatewayId]});
      showToast(t('gateways:variables.edit.success', 'Variable {{name}} updated.', {name}), 'success');
    },
  });
}
