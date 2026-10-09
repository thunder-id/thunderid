// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';
import type {ResourcePermissions} from '../models/role';

/**
 * Unassigns a role from one assignee through the configured flow; returns whether a flow ran.
 */
export async function removeRoleAssignmentViaFlow(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  assigneeId: string,
): Promise<boolean> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_ASSIGNMENT_REMOVAL,
    {
      [AdministrationFlowInput.ASSIGNEE]: assigneeId,
      [AdministrationFlowInput.ROLE]: roleId,
    },
    'role assignment removal',
  );

  return result !== null;
}

/**
 * Deletes a role through the configured flow; returns whether a flow ran.
 */
export async function deleteRoleViaFlow(http: HttpLike, serverUrl: string, roleId: string): Promise<boolean> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_DELETION,
    {[AdministrationFlowInput.ROLE]: roleId},
    'role deletion',
  );

  return result !== null;
}

/**
 * Reports whether the new permission set removes any permission. Only a removal needs the flow, which
 * revokes the role's whole scope set rather than a delta.
 */
export function permissionsRemoved(
  current: ResourcePermissions[] | undefined,
  next: ResourcePermissions[] | undefined,
): boolean {
  const retained = new Set<string>();

  for (const resource of next ?? []) {
    for (const permission of resource.permissions ?? []) {
      retained.add(`${resource.resourceServerId}\u0000${permission}`);
    }
  }

  return (current ?? []).some((resource) =>
    (resource.permissions ?? []).some((permission) => !retained.has(`${resource.resourceServerId}\u0000${permission}`)),
  );
}

/**
 * Replaces a role's permissions through the configured flow; returns whether a flow ran.
 */
export async function updateRolePermissionsViaFlow(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  permissions: ResourcePermissions[] | undefined,
): Promise<boolean> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_PERMISSION_REMOVAL,
    {
      [AdministrationFlowInput.PERMISSIONS]: JSON.stringify(permissions ?? []),
      [AdministrationFlowInput.ROLE]: roleId,
    },
    'role permission change',
  );

  return result !== null;
}
