// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AuthorizationDetailTypeQueryKeys from '../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailType} from '../models/authorization-detail-type';

export default function useGetAuthorizationDetailType(id: string): UseQueryResult<AuthorizationDetailType> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AuthorizationDetailType>({
    queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPE, id],
    queryFn: async (): Promise<AuthorizationDetailType> => {
      const serverUrl = getServerUrl();
      const response: {data: AuthorizationDetailType} = await http.request({
        url: `${serverUrl}/authorization-detail-types/${id}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
    enabled: Boolean(id),
    // A schema's properties are listed in the order the consent screen shows them, which structural
    // sharing would lose by keeping the earlier object whenever only the order changed.
    structuralSharing: false,
  });
}
