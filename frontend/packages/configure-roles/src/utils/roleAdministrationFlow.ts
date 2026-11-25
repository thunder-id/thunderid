// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {HttpLike} from '@thunderid/utils';
import type {UpdateRoleRequest} from '../models/requests';
import type {Role, RoleAssignment} from '../models/role';

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
