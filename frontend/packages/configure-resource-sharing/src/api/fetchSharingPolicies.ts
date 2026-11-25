// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {useThunderID} from '@thunderid/react';
import type {SharingPolicy, SharingPolicyListResponse} from '../models/sharing-policy';

type HttpClient = ReturnType<typeof useThunderID>['http'];

/**
 * Reads every sharing policy of one shareable resource, following pagination to the end.
 *
 * @param http - The authenticated HTTP client
 * @param serverUrl - The server's base URL
 * @param resourcePath - The resource's path relative to the server URL, e.g. `applications/<id>`
 * @returns All of the resource's policies, reshares included
 */
export default async function fetchSharingPolicies(
  http: HttpClient,
  serverUrl: string,
  resourcePath: string,
): Promise<SharingPolicy[]> {
  const policies: SharingPolicy[] = [];

  for (;;) {
    const response: {data: SharingPolicyListResponse} = await http.request({
      url: `${serverUrl}/${resourcePath}/sharing-policies?offset=${policies.length}`,
      method: 'GET',
      headers: {'Content-Type': 'application/json'},
    } as unknown as Parameters<typeof http.request>[0]);

    const page = response.data.policies ?? [];
    policies.push(...page);

    if (page.length === 0 || policies.length >= response.data.totalResults) {
      return policies;
    }
  }
}
