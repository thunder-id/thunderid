// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import type {ResourcePermissions} from '../models/resource-server';

export type SelectionState = 'all' | 'some' | 'none';

export function isPermissionSelected(
  list: ResourcePermissions[],
  resourceServerId: string,
  permission: string,
): boolean {
  return list.some((entry) => entry.resourceServerId === resourceServerId && entry.permissions.includes(permission));
}

/**
 * Lists the permissions of the resources a permission sits under, nearest first. Permission strings
 * are built as `parent + delimiter + handle` and handles may not contain the delimiter, so every
 * prefix of a permission is exactly one of its ancestors.
 */
export function ancestorPermissions(permission: string, delimiter: string): string[] {
  if (!delimiter) return [];
  const segments = permission.split(delimiter);
  return segments.slice(0, -1).map((_, index) => segments.slice(0, index + 1).join(delimiter));
}

export function togglePermission(
  list: ResourcePermissions[],
  resourceServerId: string,
  permission: string,
  delimiter: string,
): ResourcePermissions[] {
  const existing = list.find((entry) => entry.resourceServerId === resourceServerId);
  if (!existing) {
    return [...list, {resourceServerId, permissions: [permission]}];
  }

  // An ancestor confers every permission beneath it, so dropping only the permission itself would
  // leave the role still holding it through the ancestor.
  const removed = new Set([permission, ...ancestorPermissions(permission, delimiter)]);
  const updatedPermissions = existing.permissions.includes(permission)
    ? existing.permissions.filter((p) => !removed.has(p))
    : [...existing.permissions, permission];

  if (updatedPermissions.length === 0) {
    return list.filter((entry) => entry.resourceServerId !== resourceServerId);
  }

  return list.map((entry) =>
    entry.resourceServerId === resourceServerId ? {...entry, permissions: updatedPermissions} : entry,
  );
}

export function mergePermissions(base: ResourcePermissions[], additions: ResourcePermissions[]): ResourcePermissions[] {
  const result = base.map((entry) => ({...entry, permissions: [...entry.permissions]}));

  for (const addition of additions) {
    const existing = result.find((entry) => entry.resourceServerId === addition.resourceServerId);
    if (!existing) {
      result.push({resourceServerId: addition.resourceServerId, permissions: [...addition.permissions]});
      continue;
    }
    for (const permission of addition.permissions) {
      if (!existing.permissions.includes(permission)) {
        existing.permissions.push(permission);
      }
    }
  }

  return result;
}

export function removePermissions(
  list: ResourcePermissions[],
  resourceServerId: string,
  permissions: string[],
): ResourcePermissions[] {
  const entry = list.find((e) => e.resourceServerId === resourceServerId);
  if (!entry) return list;

  const updatedPermissions = entry.permissions.filter((p) => !permissions.includes(p));
  if (updatedPermissions.length === 0) {
    return list.filter((e) => e.resourceServerId !== resourceServerId);
  }

  return list.map((e) => (e.resourceServerId === resourceServerId ? {...e, permissions: updatedPermissions} : e));
}

export function getSubtreeSelectionState(
  list: ResourcePermissions[],
  resourceServerId: string,
  subtreePermissions: string[],
): SelectionState {
  if (subtreePermissions.length === 0) return 'none';
  const selectedCount = subtreePermissions.filter((p) => isPermissionSelected(list, resourceServerId, p)).length;
  if (selectedCount === 0) return 'none';
  return selectedCount === subtreePermissions.length ? 'all' : 'some';
}

export function arePermissionsEqual(a: ResourcePermissions[], b: ResourcePermissions[]): boolean {
  if (a.length !== b.length) return false;
  return a.every((entryA) => {
    const entryB = b.find((e) => e.resourceServerId === entryA.resourceServerId);
    if (entryA.permissions.length !== entryB?.permissions.length) return false;
    return entryA.permissions.every((p) => entryB.permissions.includes(p));
  });
}
