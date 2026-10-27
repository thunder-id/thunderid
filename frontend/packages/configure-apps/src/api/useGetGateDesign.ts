// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import BuilderQueryKeys from '../constants/builder-query-keys';
import type {AppGateDesign} from '../models/builder';
import toGateDesign from '../utils/toGateDesign';

/**
 * Fetches the Gate design of the application an app signs in with: the theme and layout of the
 * app's sign-in screens.
 *
 * @param applicationId - The application, as the app configuration names it. Nothing is fetched
 * without one.
 * @returns TanStack Query result object containing the Gate design.
 *
 * @public
 */
export default function useGetGateDesign(applicationId: string | undefined): UseQueryResult<AppGateDesign> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<AppGateDesign>({
    queryKey: [BuilderQueryKeys.SIGN_IN_APPLICATION, applicationId],
    queryFn: async (): Promise<AppGateDesign> => {
      const response: {data: AppGateDesign} = await http.request({
        url: `${getServerUrl()}/applications/${applicationId}`,
        method: 'GET',
      } as unknown as Parameters<typeof http.request>[0]);

      return toGateDesign(response.data);
    },
    enabled: Boolean(applicationId),
  });
}
