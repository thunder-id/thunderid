// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';
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
 * Deletes a group through the configured deletion flow, falling back to the native endpoint.
 *
 * Only the flow revokes the scopes the members held through the group; a configured flow that cannot be
 * resolved refuses.
 */
export async function deleteGroupViaFlow(http: HttpLike, serverUrl: string, groupId: string): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.GROUP_DELETION,
    {[AdministrationFlowInput.GROUP]: groupId},
    'group deletion',
  );

  if (result === null) {
    await deleteGroupNatively(http, serverUrl, groupId);
  }
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

/**
 * Removes one member from a group through the configured flow, falling back to the native endpoint.
 * One member per execution, since a revocation criterion names one principal.
 *
 * Only the flow revokes the scopes the member held through the group; a configured flow that cannot be
 * resolved refuses.
 */
export async function removeGroupMemberViaFlow(
  http: HttpLike,
  serverUrl: string,
  groupId: string,
  member: Member,
): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.GROUP_MEMBERSHIP_REMOVAL,
    {
      [AdministrationFlowInput.GROUP]: groupId,
      [AdministrationFlowInput.MEMBER]: member.id,
    },
    'group membership removal',
  );

  if (result === null) {
    await removeGroupMemberNatively(http, serverUrl, groupId, member);
  }
}
