// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';

/**
 * Routes the group changes that take scopes away through their administration flows, so the tokens
 * carrying those scopes are revoked before the change is applied.
 *
 * Membership conveys every role the group holds, including through its ancestors, so cutting it takes
 * scopes away exactly as an unassignment does. The configured-and-present rule is the same as the role
 * paths use: an unset handle keeps the native endpoint, a set handle naming no flow refuses.
 */

/**
 * Deletes a group through the configured flow, or returns false when none is configured.
 */
export async function deleteGroupViaFlow(http: HttpLike, serverUrl: string, groupId: string): Promise<boolean> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.GROUP_DELETION,
    {[AdministrationFlowInput.GROUP]: groupId},
    'group deletion',
  );

  return result !== null;
}

/**
 * Removes one member from a group through the configured flow, or returns false when none is
 * configured.
 *
 * One member per execution, because the revocation is computed for the principals losing the path and
 * a criterion names one principal. A caller removing several members runs it once per member.
 */
export async function removeGroupMemberViaFlow(
  http: HttpLike,
  serverUrl: string,
  groupId: string,
  memberId: string,
): Promise<boolean> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.GROUP_MEMBERSHIP_REMOVAL,
    {
      [AdministrationFlowInput.GROUP]: groupId,
      [AdministrationFlowInput.MEMBER]: memberId,
    },
    'group membership removal',
  );

  return result !== null;
}
