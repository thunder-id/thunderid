// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useRuntimeUrl} from '@thunderid/contexts';
import ConnectionQueryKeys from '../constants/query-keys';
import type {AuthorizationServerMetadata} from '../models/authorization-server-metadata';

/**
 * Fetch the runtime's OAuth 2.0 authorization server metadata
 * (GET `{runtimeUrl}/.well-known/oauth-authorization-server`), which says which grants and grant
 * profiles the runtime the applications call actually serves.
 *
 * The document is public, so it is fetched without credentials: an Authorization header would
 * fail its CORS preflight, which allows only Content-Type.
 *
 * @returns The query result. `data` stays undefined while loading and after a failure, so callers
 * treat a missing document as unknown rather than as a missing capability.
 *
 * @example
 * ```tsx
 * const {data: serverMetadata} = useAuthorizationServerMetadata();
 * const grantTypes = serverMetadata?.grant_types_supported;
 * ```
 *
 * @public
 */
export default function useAuthorizationServerMetadata(): UseQueryResult<AuthorizationServerMetadata> {
  const runtimeUrl = useRuntimeUrl();

  return useQuery<AuthorizationServerMetadata>({
    queryKey: [ConnectionQueryKeys.AUTHORIZATION_SERVER_METADATA, {runtimeUrl}],
    queryFn: async (): Promise<AuthorizationServerMetadata> => {
      const response = await fetch(`${runtimeUrl}/.well-known/oauth-authorization-server`);
      if (!response.ok) {
        throw new Error(`Authorization server metadata request failed with status ${response.status}`);
      }

      return (await response.json()) as AuthorizationServerMetadata;
    },
  });
}
