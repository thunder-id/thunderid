// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {ApplicationQueryKeys, getApplication, useManagementRequestConfig, type Application} from '@thunderid/react';

/**
 * Fetches the application the simulation preview is themed as.
 *
 * @param applicationId - The application to fetch. Nothing is fetched while it is empty.
 * @returns TanStack Query result holding the application.
 */
export default function useGetApplication(applicationId: string): UseQueryResult<Application> {
  const config = useManagementRequestConfig('applications');

  return useQuery<Application>({
    enabled: Boolean(applicationId),
    queryFn: (): Promise<Application> => getApplication({...config, applicationId}),
    queryKey: [ApplicationQueryKeys.APPLICATION, applicationId],
  });
}
