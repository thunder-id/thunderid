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
 * Routes the role changes that take scopes away through their administration flows, so the tokens
 * carrying those scopes are revoked before the change is applied.
 *
 * The presence of flow configuration is the switch. A deployment that leaves a handle unset keeps the
 * change on its native endpoint, which revokes nothing; that is the documented opt-out. A handle that
 * is set but names no existing flow refuses instead of falling back, because that is a broken opt-in:
 * falling back would perform the change with no revocation and report it as a success, which is the
 * one outcome the configuration was set to prevent.
 *
 * Each function returns whether a flow ran, so the caller falls back only when none is configured.
 */

/**
 * Unassigns a role from one assignee through the configured flow.
 *
 * The flow acts on one assignee at a time, because a criterion names one principal. A caller removing
 * several assignees runs it once per assignee.
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
 * Deletes a role through the configured flow.
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
 * Reports whether the new permission set takes anything away from the current one.
 *
 * Only a removal needs revoking. Adding a permission, or renaming the role, takes no scope from
 * anyone, and running the flow for those would revoke every scope the role grants for every holder to
 * no purpose, since the flow deliberately revokes the whole set rather than a delta.
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
 * Replaces the permissions a role grants through the configured flow.
 *
 * The permission set travels as a JSON string because every flow input is text. The flow revokes every
 * scope the role grants today rather than the delta this set implies, so a principal that keeps a
 * scope through this role pays one token refresh for it.
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
