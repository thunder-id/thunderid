// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import VerifiablePresentationQueryKeys from '../constants/vp-query-keys';
import type {VPListResponse} from '../models/vp';

/**
 * Fetches the OpenID4VP presentation definitions: every definition, or only a project's when a project ID is
 * given.
 *
 * @param projectId - List only the presentation definitions of this project
 */
export default function useGetVerifiablePresentations(projectId?: string): UseQueryResult<VPListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<VPListResponse>({
    queryKey: [VerifiablePresentationQueryKeys.VPS, ...(projectId ? [{projectId}] : [])],
    queryFn: async (): Promise<VPListResponse> => {
      const serverUrl: string = getServerUrl();
      const query: string = projectId ? `?${new URLSearchParams({projectId}).toString()}` : '';
      const response: {data: VPListResponse} = await http.request({
        url: `${serverUrl}/openid4vp/presentation-definitions${query}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data ?? [];
    },
  });
}
