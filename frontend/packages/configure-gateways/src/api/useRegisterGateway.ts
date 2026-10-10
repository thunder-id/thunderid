// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {GatewayRegistration, RegisterGatewayRequest} from '../models/gateway';

export default function useRegisterGateway(): UseMutationResult<GatewayRegistration, Error, RegisterGatewayRequest> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const queryClient = useQueryClient();

  return useMutation<GatewayRegistration, Error, RegisterGatewayRequest>({
    mutationFn: async (data: RegisterGatewayRequest): Promise<GatewayRegistration> => {
      const response: {data: GatewayRegistration} = await http.request({
        url: `${getServerUrl()}/gateways`,
        method: 'POST',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: () => {
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAYS]});
    },
  });
}
