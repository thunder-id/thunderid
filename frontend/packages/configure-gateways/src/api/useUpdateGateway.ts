// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {Gateway, UpdateGatewayRequest} from '../models/gateway';

export default function useUpdateGateway(): UseMutationResult<
  Gateway,
  Error,
  {id: string; data: UpdateGatewayRequest}
> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {showToast} = useToast();
  const {t} = useTranslation();
  const queryClient = useQueryClient();

  return useMutation<Gateway, Error, {id: string; data: UpdateGatewayRequest}>({
    mutationFn: async ({id, data}): Promise<Gateway> => {
      const response: {data: Gateway} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(id)}`,
        method: 'PUT',
        data,
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    onSuccess: (gateway, {id, data}) => {
      queryClient.setQueryData([GatewayQueryKeys.GATEWAY, id], gateway);
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAYS]});
      showToast(
        data.key !== undefined
          ? t('gateways:rotateKey.success', 'The key was rotated.')
          : t('gateways:edit.success', 'Gateway updated.'),
        'success',
      );
    },
  });
}
