// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import GatewayQueryKeys from '../constants/gateway-query-keys';
import type {AppliedVersion} from '../models/gateway';

export default function useGetAppliedVersion(gatewayId: string): UseQueryResult<AppliedVersion> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AppliedVersion>({
    queryKey: [GatewayQueryKeys.APPLIED_VERSION, gatewayId],
    queryFn: async (): Promise<AppliedVersion> => {
      const response: {data: AppliedVersion} = await http.request({
        url: `${getServerUrl()}/gateways/${encodeURIComponent(gatewayId)}/applied-version`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data;
    },
    enabled: Boolean(gatewayId),
  });
}
