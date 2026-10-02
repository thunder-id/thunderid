// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayQueryKeys from '../constants/gateway-query-keys';

export default function useDeleteGatewaySecret(): UseMutationResult<void, Error, {gatewayId: string; name: string}> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {showToast} = useToast();
  const {t} = useTranslation();
  const queryClient = useQueryClient();

  return useMutation<void, Error, {gatewayId: string; name: string}>({
    mutationFn: async ({gatewayId, name}): Promise<void> => {
      await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/secrets/${encodeURIComponent(name)}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result, {gatewayId, name}) => {
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_SECRETS, gatewayId]});
      // What the gateway holds decides what an apply or a revert would find missing.
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAY_DRY_RUN, gatewayId]});
      showToast(t('gateways:secrets.delete.success', 'Secret {{name}} deleted.', {name}), 'success');
    },
  });
}
