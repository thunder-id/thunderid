// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useQuery, type UseQueryResult} from '@tanstack/react-query';
import {useConfig} from '@thunderid/contexts';
import {useThunderID} from '@thunderid/react';
import fetchSharingPolicies from './fetchSharingPolicies';
import ResourceSharingQueryKeys from '../constants/resource-sharing-query-keys';
import type {SharingPolicy} from '../models/sharing-policy';

/**
 * Fetches every sharing policy of one shareable resource, following pagination to the end.
 *
 * Every resource that mounts sharing endpoints serves them under its own path, so the hook takes
 * that path rather than knowing any resource type.
 *
 * @param resourcePath - The resource's path relative to the server URL, e.g. `applications/<id>`
 * @param enabled - Whether to fetch
 * @returns TanStack Query result holding all of the resource's policies, reshares included
 *
 * @example
 * ```tsx
 * const {data: policies} = useGetSharingPolicies(`applications/${applicationId}`);
 * ```
 *
 * @public
 */
export default function useGetSharingPolicies(resourcePath: string, enabled = true): UseQueryResult<SharingPolicy[]> {
  const {http} = useThunderID();
  const {getServerUrl} = useConfig();

  return useQuery<SharingPolicy[]>({
    queryKey: [ResourceSharingQueryKeys.SHARING_POLICIES, resourcePath],
    queryFn: (): Promise<SharingPolicy[]> => fetchSharingPolicies(http, getServerUrl(), resourcePath),
    enabled: enabled && Boolean(resourcePath),
  });
}
