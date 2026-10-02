// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {Gateway} from '../models/gateway';

export default function useGetGateway(id: string): UseQueryResult<Gateway> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Gateway>({
    queryKey: [GatewayQueryKeys.GATEWAY, id],
    queryFn: async (): Promise<Gateway> => {
      const response: {data: Gateway} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(id)}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(id),
  });
}
