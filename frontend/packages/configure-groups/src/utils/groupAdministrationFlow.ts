// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {HttpLike} from '@thunderid/utils';
import type {Member} from '../models/group';

export type {HttpLike};

/**
 * Deletes a group through the native endpoint.
 */
export async function deleteGroupNatively(http: HttpLike, serverUrl: string, groupId: string): Promise<void> {
  await http.request({
    url: `${serverUrl}/groups/${groupId}`,
    method: 'DELETE',
  });
}

/**
 * Removes one member from a group through the native endpoint.
 */
export async function removeGroupMemberNatively(
  http: HttpLike,
  serverUrl: string,
  groupId: string,
  member: Member,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/groups/${groupId}/members/remove`,
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    data: {members: [member]},
  });
}
