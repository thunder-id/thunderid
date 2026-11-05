// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {HttpLike} from '@thunderid/utils';

export type {HttpLike};

/**
 * Deletes an action through the native endpoint. `resourceId` is omitted for an action defined on the
 * resource server itself.
 */
export async function deleteActionNatively(
  http: HttpLike,
  serverUrl: string,
  resourceServerId: string,
  actionId: string,
  resourceId?: string,
): Promise<void> {
  const url = resourceId
    ? `${serverUrl}/resource-servers/${resourceServerId}/resources/${resourceId}/actions/${actionId}`
    : `${serverUrl}/resource-servers/${resourceServerId}/actions/${actionId}`;

  await http.request({
    url,
    method: 'DELETE',
  });
}

