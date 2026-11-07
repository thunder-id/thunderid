// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useRuntimeUrl} from '@thunderid/contexts';
import ApplicationQueryKeys from '../constants/application-query-keys';
import type {OpenIDConfiguration} from '../models/openid-configuration';

/**
 * Fetch the OpenID Connect discovery document of the runtime the applications call
 * (GET `{runtimeUrl}/.well-known/openid-configuration`).
 *
 * The document is public, so it is fetched without credentials.
 *
 * @returns The query result. `data` stays undefined while loading and after a failure.
 *
 * @example
 * ```tsx
 * const {data: openIdConfiguration} = useGetOpenIDConfiguration();
 * const grantTypes = openIdConfiguration?.grant_types_supported ?? [];
 * ```
 *
 * @public
 */
export default function useGetOpenIDConfiguration(): UseQueryResult<OpenIDConfiguration> {
  const runtimeUrl = useRuntimeUrl();

  return useQuery<OpenIDConfiguration>({
    queryKey: [ApplicationQueryKeys.OPENID_CONFIGURATION, {runtimeUrl}],
    queryFn: async (): Promise<OpenIDConfiguration> => {
      const response = await fetch(`${runtimeUrl}/.well-known/openid-configuration`);
      if (!response.ok) {
        throw new Error(`OpenID configuration request failed with status ${response.status}`);
      }

      return (await response.json()) as OpenIDConfiguration;
    },
  });
}
