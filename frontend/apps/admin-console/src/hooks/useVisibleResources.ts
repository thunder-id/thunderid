// Copyright 2026 The ThunderID Authors
// SPDX-License-Identifier: Apache-2.0

import {useGrantedPermissions} from '@thunderid/react';
import {MANAGED_RESOURCES, type ManagedResource} from '../configs/RouteConfig';

/**
 * The permission a page needs: an admin whose token lacks it never sees the page. The server still
 * decides every request.
 */
const VIEW_PERMISSIONS: Record<ManagedResource, string> = {
  agents: 'system:agent:view',
  groups: 'system:group:view',
  roles: 'system:role:view',
  users: 'system:user:view',
};

/**
 * Reading the configured administration flows needs the server configuration, which only the root
 * `system` permission may read. Lifecycle actions are offered only to admins who hold it.
 */
const LIFECYCLE_PERMISSION = 'system';

export interface VisibleResources {
  /**
   * Whether the admin may suspend, unsuspend and unlock users and agents.
   */
  canRunLifecycle: boolean;
  /**
   * Whether the admin's permissions are still loading.
   */
  isLoading: boolean;
  /**
   * The pages this admin may see, in navigation order.
   */
  visible: ManagedResource[];
}

/**
 * Decides which pages the signed-in admin sees, from the permissions their token carries.
 */
export default function useVisibleResources(): VisibleResources {
  const {can, isLoading} = useGrantedPermissions();

  return {
    canRunLifecycle: can(LIFECYCLE_PERMISSION),
    isLoading,
    visible: MANAGED_RESOURCES.filter((resource: ManagedResource) => can(VIEW_PERMISSIONS[resource])),
  };
}
