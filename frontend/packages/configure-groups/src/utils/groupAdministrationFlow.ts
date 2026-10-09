// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';

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
 * configured. One member per execution, since a revocation criterion names one principal.
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
