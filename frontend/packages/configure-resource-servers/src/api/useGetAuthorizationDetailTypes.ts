// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {keepPreviousData, useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import AuthorizationDetailTypeQueryKeys from '../constants/authorization-detail-type-query-keys';
import type {AuthorizationDetailTypeListResponse} from '../models/authorization-detail-type';

export default function useGetAuthorizationDetailTypes(params?: {
  limit?: number;
  offset?: number;
}): UseQueryResult<AuthorizationDetailTypeListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();
  const {limit = 100, offset = 0} = params ?? {};

  return useQuery<AuthorizationDetailTypeListResponse>({
    queryKey: [AuthorizationDetailTypeQueryKeys.AUTHORIZATION_DETAIL_TYPES, {limit, offset}],
    placeholderData: keepPreviousData,
    queryFn: async (): Promise<AuthorizationDetailTypeListResponse> => {
      const serverUrl = getServerUrl();
      const queryParams = new URLSearchParams({limit: limit.toString(), offset: offset.toString()});
      const response: {data: AuthorizationDetailTypeListResponse} = await http.request({
        url: `${serverUrl}/authorization-detail-types?${queryParams.toString()}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);
      return response.data;
    },
  });
}
