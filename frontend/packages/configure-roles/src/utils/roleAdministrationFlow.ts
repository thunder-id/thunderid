// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {
  AdministrationFlowConfigKey,
  AdministrationFlowInput,
  executeAdministrationFlow,
  resolveAdministrationFlowHandle,
  resolveAdministrationFlowId,
  runAdministrationFlow,
  type HttpLike,
} from '@thunderid/utils';
import type {UpdateRoleRequest} from '../models/requests';
import type {ResourcePermissions, Role, RoleAssignment} from '../models/role';

export type {HttpLike};

/**
 * Deletes a role through the native endpoint.
 */
export async function deleteRoleNatively(http: HttpLike, serverUrl: string, roleId: string): Promise<void> {
  await http.request({
    url: `${serverUrl}/roles/${roleId}`,
    method: 'DELETE',
  });
}

/**
 * Deletes a role through the configured deletion flow, falling back to the native endpoint.
 *
 * Only the flow revokes the scopes the role granted; a configured flow that cannot be resolved refuses.
 */
export async function deleteRoleViaFlow(http: HttpLike, serverUrl: string, roleId: string): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_DELETION,
    {[AdministrationFlowInput.ROLE]: roleId},
    'role deletion',
  );

  if (result === null) {
    await deleteRoleNatively(http, serverUrl, roleId);
  }
}

/**
 * Unassigns a role from one assignee through the native endpoint.
 */
export async function removeRoleAssignmentNatively(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  assignment: RoleAssignment,
): Promise<void> {
  await http.request({
    url: `${serverUrl}/roles/${roleId}/assignments/remove`,
    method: 'POST',
    headers: {'Content-Type': 'application/json'},
    data: {assignments: [assignment]},
  });
}

/**
 * Unassigns a role from one assignee through the configured flow, falling back to the native endpoint.
 *
 * Only the flow revokes the scopes the assignment conveyed; a configured flow that cannot be resolved
 * refuses.
 */
export async function removeRoleAssignmentViaFlow(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  assignment: RoleAssignment,
): Promise<void> {
  const result = await runAdministrationFlow(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_ASSIGNMENT_REMOVAL,
    {
      [AdministrationFlowInput.ASSIGNEE]: assignment.id,
      [AdministrationFlowInput.ROLE]: roleId,
    },
    'role assignment removal',
  );

  if (result === null) {
    await removeRoleAssignmentNatively(http, serverUrl, roleId, assignment);
  }
}

/**
 * Reports whether the new permission set removes any permission. Only a removal has anything to revoke.
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
 * Updates a role through the native endpoint.
 */
export async function updateRoleNatively(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  data: UpdateRoleRequest,
): Promise<Role> {
  const response = await http.request({
    url: `${serverUrl}/roles/${roleId}`,
    method: 'PUT',
    headers: {'Content-Type': 'application/json'},
    data,
  });

  return response.data as Role;
}

/**
 * Updates a role, running a permission removal through the configured flow and everything else through
 * the native endpoint.
 *
 * `current` is the role as the caller last read it; it decides whether anything is removed.
 */
export async function updateRoleViaFlow(
  http: HttpLike,
  serverUrl: string,
  roleId: string,
  data: UpdateRoleRequest,
  current: Role,
): Promise<Role> {
  if (!permissionsRemoved(current.permissions, data.permissions)) {
    return updateRoleNatively(http, serverUrl, roleId, data);
  }
  const handle = await resolveAdministrationFlowHandle(
    http,
    serverUrl,
    AdministrationFlowConfigKey.ROLE_PERMISSION_REMOVAL,
  );

  if (!handle) {
    return updateRoleNatively(http, serverUrl, roleId, data);
  }

  const flowId = await resolveAdministrationFlowId(http, serverUrl, handle, 'role permission change');

  // The flow writes only permissions and the writes are not atomic, so other fields go first and a
  // rejected rename fails before anything is revoked.
  const otherFieldsChanged: boolean =
    data.name !== current.name ||
    (data.description ?? '') !== (current.description ?? '') ||
    data.ouId !== current.ouId;
  const updated: Role = otherFieldsChanged
    ? await updateRoleNatively(http, serverUrl, roleId, {...data, permissions: current.permissions ?? []})
    : current;

  await executeAdministrationFlow(
    http,
    serverUrl,
    flowId,
    {
      [AdministrationFlowInput.PERMISSIONS]: JSON.stringify(data.permissions ?? []),
      [AdministrationFlowInput.ROLE]: roleId,
    },
    'role permission change',
  );

  return {...updated, permissions: data.permissions};
}
