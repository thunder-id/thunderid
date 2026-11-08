// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import VerifiableCredentialQueryKeys from '../constants/vc-query-keys';
import type {VCListResponse} from '../models/vc';

/**
 * Fetches the OpenID4VCI credential configurations: every configuration, or only a project's when a project ID is
 * given.
 *
 * @param projectId - List only the credential configurations of this project
 */
export default function useGetVerifiableCredentials(projectId?: string): UseQueryResult<VCListResponse> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<VCListResponse>({
    queryKey: [VerifiableCredentialQueryKeys.VCS, ...(projectId ? [{projectId}] : [])],
    queryFn: async (): Promise<VCListResponse> => {
      const serverUrl: string = getServerUrl();
      const query: string = projectId ? `?${new URLSearchParams({projectId}).toString()}` : '';
      const response: {data: VCListResponse} = await http.request({
        url: `${serverUrl}/openid4vci/credential-configurations${query}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return response.data ?? [];
    },
  });
}
