// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {Gateway} from '../models/gateway';

export default function useGetGateways(): UseQueryResult<Gateway[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<Gateway[]>({
    queryKey: [GatewayQueryKeys.GATEWAYS],
    queryFn: async (): Promise<Gateway[]> => {
      const response: {data: Gateway[] | null} = await http.request({
        url: `${getServerUrl()}/gateways`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data ?? [];
    },
  });
}
