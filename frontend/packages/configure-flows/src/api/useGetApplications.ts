// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {
  ApplicationQueryKeys,
  getApplications,
  useManagementRequestConfig,
  type ApplicationListResponse,
} from '@thunderid/react';

/**
 * Fetches the applications the simulation preview can theme itself as.
 *
 * @returns TanStack Query result holding the first page of applications.
 */
export default function useGetApplications(): UseQueryResult<ApplicationListResponse> {
  const config = useManagementRequestConfig('applications');

  return useQuery<ApplicationListResponse>({
    queryFn: (): Promise<ApplicationListResponse> => getApplications(config),
    queryKey: [ApplicationQueryKeys.APPLICATIONS],
  });
}
