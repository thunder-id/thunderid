// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import BuilderQueryKeys from '../constants/builder-query-keys';

/**
 * Fetches the logo of the application an app signs in with: the logo the Gate shows, and the one
 * the app shows until it is given its own.
 *
 * @param applicationId - The application. Nothing is fetched without one.
 * @returns TanStack Query result object containing the logo URL, or `null` when the application has none.
 *
 * @public
 */
export default function useGetSignInApplicationLogo(applicationId: string | undefined): UseQueryResult<string | null> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<string | null>({
    queryKey: [BuilderQueryKeys.SIGN_IN_APPLICATION_LOGO, applicationId],
    queryFn: async (): Promise<string | null> => {
      const response: {data: {logoUrl?: string}} = await http.request({
        url: `${getServerUrl()}/applications/${applicationId}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      // The server leaves out an unset logo.
      return response.data.logoUrl ?? null;
    },
    enabled: Boolean(applicationId),
  });
}
