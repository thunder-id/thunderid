// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {CreateGatewayValueRequest, GatewayVariable} from '../models/gateway';

export default function useCreateGatewayVariable(): UseMutationResult<
  GatewayVariable,
  Error,
  {gatewayId: string; data: CreateGatewayValueRequest}
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {showToast} = useToast();
  const {t} = useTranslation();
  const queryClient = useQueryClient();

  return useMutation<GatewayVariable, Error, {gatewayId: string; data: CreateGatewayValueRequest}>({
    mutationFn: async ({gatewayId, data}): Promise<GatewayVariable> => {
      const response: {data: GatewayVariable} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/variables`,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (_result, {gatewayId, data}) => {
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_VARIABLES, gatewayId]});
      // What the gateway holds decides what an apply or a revert would find missing.
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, gatewayId]});
      showToast(t('gateways:variables.create.success', 'Variable {{name}} added.', {name: data.name}), 'success');
    },
  });
}
