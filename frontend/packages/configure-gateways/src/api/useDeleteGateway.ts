// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useMutation, useQueryClient, type UseMutationResult} from '@tanstack/react-query';
import {useConfig, useToast} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import {useTranslation} from 'react-i18next';
import GatewayQueryKeys from '../constants/gateway-query-keys';

export default function useDeleteGateway(): UseMutationResult<void, Error, string> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {showToast} = useToast();
  const {t} = useTranslation();
  const queryClient = useQueryClient();

  return useMutation<void, Error, string>({
    mutationFn: async (id: string): Promise<void> => {
      await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(id)}`,
        method: 'DELETE',
      } as unknown as Parameters<typeof http.request>[0]);
    },
    onSuccess: (_result, id) => {
      queryClient.removeQueries({queryKey: [GatewayQueryKeys.GATEWAY, id]});
      void queryClient.invalidateQueries({queryKey: [GatewayQueryKeys.GATEWAYS]});
      showToast(t('gateways:delete.success', 'Gateway removed.'), 'success');
    },
  });
}
